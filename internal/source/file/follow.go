package file

import (
	"context"
	"errors"
	"io"
	"time"

	"github.com/Coubiac/mailtrace/internal/source"
)

const (
	DefaultPollInterval = time.Second
	MinPollInterval     = 10 * time.Millisecond
	MaxPollInterval     = time.Minute
)

// Follow consumes additions to this already opened generation until cancelled
// or an error occurs. Zero selects DefaultPollInterval; explicit intervals must
// be within [MinPollInterval, MaxPollInterval]. Only a reader EOF causes waiting:
// all Sink errors, including EOF, are returned with the pending batch retained.
//
// One caller must own this ingestor and descriptor. Follow does not close the
// file, choose generations, inspect the path or detect rotation/truncation.
// Cancellation remains subject to the bounded reads and Sink's context support.
func (r *Ingestor) Follow(ctx context.Context, sink source.Sink, pollInterval time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if sink == nil {
		return errors.New("sink is required")
	}
	interval, err := resolvePollInterval(pollInterval)
	if err != nil {
		return err
	}
	return r.follow(ctx, sink, func(ctx context.Context) error { return waitForPoll(ctx, interval) })
}

func resolvePollInterval(interval time.Duration) (time.Duration, error) {
	if interval == 0 {
		interval = DefaultPollInterval
	}
	if interval < MinPollInterval || interval > MaxPollInterval {
		return 0, errors.New("poll interval must be between 10ms and 1m")
	}
	return interval, nil
}

func waitForPoll(ctx context.Context, interval time.Duration) error {
	timer := time.NewTimer(interval)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (r *Ingestor) follow(ctx context.Context, sink source.Sink, wait func(context.Context) error) error {
	for {
		err := r.CommitNext(ctx, sink)
		if err == nil {
			continue
		}
		// Sink EOF must not masquerade as a reader EOF and trigger retries.
		if !errors.Is(err, io.EOF) || r.pending != nil {
			return err
		}
		if err := wait(ctx); err != nil {
			return err
		}
	}
}
