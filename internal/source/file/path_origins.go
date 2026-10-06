package file

import (
	"context"
	"errors"

	"github.com/Coubiac/QueueAtlas/internal/source"
)

const MaxPathOrigins = 1000

type PathOriginsStatus string

const (
	PathOriginsComplete PathOriginsStatus = "complete"
	PathOriginsAbsent   PathOriginsStatus = "absent"
	PathOriginsLimit    PathOriginsStatus = "limit_reached"
)

var ErrInvalidPathOriginPage = errors.New("invalid path origin page")

type PathOrigins struct {
	Status   PathOriginsStatus
	States   []source.OriginState // owned copies, populated only for Complete
	Examined int
}

// LoadPathOrigins validates a complete paginated scan of one source and exact
// stored path. limit (1..MaxPathOrigins) bounds total states and page requests.
// A limit with a continuation cursor returns no usable states. Errors and
// cancellation discard the entire result. Checkpoints are copied as pages arrive,
// so a reader may reuse its buffers between calls without aliasing the result.
//
// The reader owns source filtering (OriginState does not contain a source ID).
// Paths are not normalized. ID ordering is not chronology; checkpoint content
// and file presence are not verified. No generation is chosen, file opened or
// state written. The caller must serialize source state writes during the scan
// and its application: independently committed pages are not a global snapshot.
func LoadPathOrigins(ctx context.Context, sourceID, path string, reader source.PathStateReader, limit int) (PathOrigins, error) {
	if err := ctx.Err(); err != nil {
		return PathOrigins{}, err
	}
	if sourceID == "" || path == "" || reader == nil || limit < 1 || limit > MaxPathOrigins {
		return PathOrigins{}, errors.New("source ID, path, reader and bounded origin limit are required")
	}
	query := source.OriginPathQuery{SourceID: sourceID, Path: path}
	states := make([]source.OriginState, 0, min(limit, source.MaxOriginPageSize))
	for {
		if err := ctx.Err(); err != nil {
			return PathOrigins{}, err
		}
		query.Limit = min(source.MaxOriginPageSize, limit-len(states))
		page, err := reader.FileOriginsByPath(ctx, query)
		if err != nil {
			return PathOrigins{}, err
		}
		if err := ctx.Err(); err != nil {
			return PathOrigins{}, err
		}
		if err := validatePathOriginPage(page, query); err != nil {
			return PathOrigins{}, err
		}
		for _, state := range page.States {
			if err := ctx.Err(); err != nil {
				return PathOrigins{}, err
			}
			if state.Checkpoint != nil {
				position := *state.Checkpoint
				state.Checkpoint = &position
			}
			states = append(states, state)
		}
		if err := ctx.Err(); err != nil {
			return PathOrigins{}, err
		}
		if page.NextID == "" {
			if len(states) == 0 {
				return PathOrigins{Status: PathOriginsAbsent}, nil
			}
			return PathOrigins{Status: PathOriginsComplete, States: states, Examined: len(states)}, nil
		}
		if len(states) == limit {
			return PathOrigins{Status: PathOriginsLimit, Examined: len(states)}, nil
		}
		query.AfterID = page.NextID
	}
}

func validatePathOriginPage(page source.OriginPage, query source.OriginPathQuery) error {
	if len(page.States) > query.Limit {
		return ErrInvalidPathOriginPage
	}
	previous := query.AfterID
	for _, state := range page.States {
		if state.Origin.ID <= previous || state.Origin.Path != query.Path {
			return ErrInvalidPathOriginPage
		}
		previous = state.Origin.ID
	}
	if page.NextID != "" && (len(page.States) == 0 || page.NextID != previous) {
		return ErrInvalidPathOriginPage
	}
	return nil
}
