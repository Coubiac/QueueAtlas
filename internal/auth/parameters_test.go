package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestDefaultParametersIndependentAndExplicit(t *testing.T) {
	want := Parameters{MemoryKiB: 65536, Iterations: 3, Parallelism: 4}
	p := DefaultParameters()
	if p != want || p.Validate() != nil || Argon2Version != 19 || SaltBytes != 16 || KeyBytes != 32 {
		t.Fatal("unexpected default hash profile")
	}
	p.MemoryKiB, p.Iterations, p.Parallelism = 0, 0, 0
	if DefaultParameters() != want || !errors.Is(p.Validate(), ErrInvalidParameters) {
		t.Fatal("defaults borrowed caller state or explicit zeros accepted")
	}
}

func TestParametersBoundsAlignmentAndNoMutation(t *testing.T) {
	for _, memory := range []uint32{65536, 262144} {
		for _, iterations := range []uint32{3, 6} {
			for _, parallelism := range []uint8{1, 2, 4} {
				p := Parameters{MemoryKiB: memory, Iterations: iterations, Parallelism: parallelism}
				before := p
				err := p.Validate()
				if p != before || err != nil {
					t.Fatal("inclusive bounds or alignment incorrect", err)
				}
			}
		}
	}
	// 65544 KiB is already aligned for three lanes; the default 65536 is not.
	if err := (Parameters{MemoryKiB: 65544, Iterations: 3, Parallelism: 3}).Validate(); err != nil {
		t.Fatal("explicit three-lane profile refused", err)
	}
	for _, tc := range []struct {
		field string
		set   func(*Parameters)
	}{
		{"memory_kib", func(p *Parameters) { p.MemoryKiB = 0 }},
		{"memory_kib", func(p *Parameters) { p.MemoryKiB = 65535 }},
		{"memory_kib", func(p *Parameters) { p.MemoryKiB = 262145 }},
		{"memory_kib", func(p *Parameters) { p.MemoryKiB = ^uint32(0) }},
		{"memory_kib", func(p *Parameters) { p.MemoryKiB = 65537 }},
		{"memory_kib", func(p *Parameters) { p.Parallelism = 3 }},
		{"iterations", func(p *Parameters) { p.Iterations = 0 }},
		{"iterations", func(p *Parameters) { p.Iterations = 2 }},
		{"iterations", func(p *Parameters) { p.Iterations = 7 }},
		{"iterations", func(p *Parameters) { p.Iterations = ^uint32(0) }},
		{"parallelism", func(p *Parameters) { p.Parallelism = 0 }},
		{"parallelism", func(p *Parameters) { p.Parallelism = 5 }},
		{"parallelism", func(p *Parameters) { p.Parallelism = ^uint8(0) }},
	} {
		p := DefaultParameters()
		tc.set(&p)
		before := p
		err := p.Validate()
		if !errors.Is(err, ErrInvalidParameters) || p != before || !strings.Contains(err.Error(), tc.field+":") {
			t.Fatal("invalid costs accepted or mutated", err)
		}
	}
}
