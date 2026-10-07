package auth

import (
	"errors"
	"fmt"
)

const (
	// Fixed lengths/version of the local Argon2id credential codec.
	Argon2Version = 0x13
	SaltBytes     = 16
	KeyBytes      = 32

	MinMemoryKiB   uint32 = 64 * 1024
	MaxMemoryKiB   uint32 = 256 * 1024
	MinIterations  uint32 = 3
	MaxIterations  uint32 = 6
	MinParallelism uint8  = 1
	MaxParallelism uint8  = 4
)

// ErrInvalidParameters reports application policy violations without supplied
// values. No hashing or resource allocation is attempted during validation.
var ErrInvalidParameters = errors.New("invalid password hash parameters")

// Parameters contains Argon2id costs, with memory in KiB (not bytes). These are
// application bounds, not the full algorithm domain or a process memory limit.
// Callers must Validate before any future hashing or credential verification.
type Parameters struct {
	MemoryKiB   uint32
	Iterations  uint32
	Parallelism uint8
}

// DefaultParameters selects RFC 9106 section 4's second recommended profile:
// 64 MiB, three passes and four lanes. Each call returns an independent value.
func DefaultParameters() Parameters {
	return Parameters{MemoryKiB: MinMemoryKiB, Iterations: MinIterations, Parallelism: MaxParallelism}
}

// Validate checks explicit costs without selecting defaults or mutating inputs.
// Memory must be a multiple of 4*parallelism to avoid implicit Argon2 rounding.
// Zero values are invalid. The upper bounds limit one invocation, not concurrency
// or elapsed time; admission controls are required at the future login boundary.
func (p Parameters) Validate() error {
	if p.MemoryKiB < MinMemoryKiB || p.MemoryKiB > MaxMemoryKiB {
		return fmt.Errorf("%w: memory_kib: expected 65536..262144 KiB", ErrInvalidParameters)
	}
	if p.Iterations < MinIterations || p.Iterations > MaxIterations {
		return fmt.Errorf("%w: iterations: expected 3..6", ErrInvalidParameters)
	}
	if p.Parallelism < MinParallelism || p.Parallelism > MaxParallelism {
		return fmt.Errorf("%w: parallelism: expected 1..4", ErrInvalidParameters)
	}
	if p.MemoryKiB%(4*uint32(p.Parallelism)) != 0 {
		return fmt.Errorf("%w: memory_kib: expected a multiple of four times parallelism", ErrInvalidParameters)
	}
	return nil
}
