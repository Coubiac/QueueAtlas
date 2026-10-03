package file

import (
	"context"
	"errors"
	"os"

	"github.com/Coubiac/mailtrace/internal/source"
)

// MaxResumeCandidates bounds both work and the number of pages in one scan.
const MaxResumeCandidates = 1000

type SelectionStatus string

const (
	SelectionUnique       SelectionStatus = "unique"
	SelectionAbsent       SelectionStatus = "absent"
	SelectionDifferent    SelectionStatus = "different"
	SelectionInsufficient SelectionStatus = "insufficient"
	SelectionAmbiguous    SelectionStatus = "ambiguous"
	SelectionLimit        SelectionStatus = "limit_reached"
)

// ErrInvalidOriginPage indicates that a reader violated ordering, pagination,
// page size or physical identity constraints. It contains no stored content.
var ErrInvalidOriginPage = errors.New("invalid origin page")

type ResumeSelection struct {
	Status    SelectionStatus
	Candidate *source.OriginState // populated only for SelectionUnique
	Examined  int
}

// SelectResume checks candidates in one source namespace without seeking or
// writing state. Unique requires an exhausted scan, exactly one match and no
// insufficient evidence. It never chooses by ID, timestamp or largest offset.
//
// The caller must serialize origin/checkpoint writes for this source throughout
// selection and application of the result: StateReader pages are not a global
// snapshot. Bounded fingerprints and concurrent file rewrites retain the limits
// documented by VerifyCandidate. Cancellation is checked between bounded reads.
func SelectResume(ctx context.Context, f *os.File, sourceID string, reader source.StateReader) (ResumeSelection, error) {
	if err := ctx.Err(); err != nil {
		return ResumeSelection{}, err
	}
	if sourceID == "" || reader == nil {
		return ResumeSelection{}, errors.New("source ID and state reader are required")
	}
	id, err := Inspect(f)
	if err != nil {
		return ResumeSelection{}, err
	}
	if err := ctx.Err(); err != nil {
		return ResumeSelection{}, err
	}
	if id.Device == "" || id.Inode == "" {
		return ResumeSelection{Status: SelectionInsufficient}, nil
	}
	query := source.OriginQuery{SourceID: sourceID, Device: id.Device, Inode: id.Inode, Limit: source.MaxOriginPageSize}
	return scanResume(ctx, reader, query, func(state source.OriginState) (ResumeCheck, error) {
		return VerifyCandidate(f, state)
	})
}

func scanResume(ctx context.Context, reader source.StateReader, query source.OriginQuery, verify func(source.OriginState) (ResumeCheck, error)) (ResumeSelection, error) {
	result := ResumeSelection{}
	var candidate *source.OriginState
	matches, insufficient := 0, false
	for {
		if err := ctx.Err(); err != nil {
			return ResumeSelection{}, err
		}
		query.Limit = min(source.MaxOriginPageSize, MaxResumeCandidates-result.Examined)
		page, err := reader.FileOrigins(ctx, query)
		if err != nil {
			return ResumeSelection{}, err
		}
		if err := validateOriginPage(page, query); err != nil {
			return ResumeSelection{}, err
		}
		for _, state := range page.States {
			if err := ctx.Err(); err != nil {
				return ResumeSelection{}, err
			}
			check, err := verify(state)
			if err != nil {
				return ResumeSelection{}, err
			}
			result.Examined++
			switch check.Status {
			case ResumeMatch:
				matches++
				if matches == 1 {
					copyState := state
					if state.Checkpoint != nil {
						copyPosition := *state.Checkpoint
						copyState.Checkpoint = &copyPosition
					}
					candidate = &copyState
				}
			case ResumeInsufficient:
				insufficient = true
			case ResumeDifferent:
			default:
				return ResumeSelection{}, errors.New("invalid resume check status")
			}
		}
		if err := ctx.Err(); err != nil {
			return ResumeSelection{}, err
		}
		if matches > 1 {
			result.Status = SelectionAmbiguous
			return result, nil
		}
		if page.NextID == "" {
			switch {
			case insufficient:
				result.Status = SelectionInsufficient
			case matches == 1:
				result.Status, result.Candidate = SelectionUnique, candidate
			case result.Examined == 0:
				result.Status = SelectionAbsent
			default:
				result.Status = SelectionDifferent
			}
			return result, nil
		}
		if result.Examined == MaxResumeCandidates {
			result.Status = SelectionLimit
			return result, nil
		}
		query.AfterID = page.NextID
	}
}

func validateOriginPage(page source.OriginPage, query source.OriginQuery) error {
	if len(page.States) > query.Limit {
		return ErrInvalidOriginPage
	}
	previous := query.AfterID
	for _, state := range page.States {
		o := state.Origin
		if o.ID <= previous || o.Device != query.Device || o.Inode != query.Inode {
			return ErrInvalidOriginPage
		}
		previous = o.ID
	}
	if page.NextID != "" && (len(page.States) == 0 || page.NextID != previous) {
		return ErrInvalidOriginPage
	}
	return nil
}
