package auth

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"golang.org/x/crypto/argon2"
)

// Independently generated with argon2-cffi 25.1.0 (reference libargon2):
// password="synthetic secret phrase", salt="0123456789abcdef", ID/v19,
// memory_cost=65536, time_cost=3, parallelism=4, hash_len=32. Synthetic only.
const syntheticPasswordHash = "$argon2id$v=19$m=65536,t=3,p=4$MDEyMzQ1Njc4OWFiY2RlZg$U7bF6tMN+ytOCGfrrY1N5M+a10UUftgXbiHnZVmZ29U"

func TestPasswordPolicyBoundsAndLiteralBytes(t *testing.T) {
	for _, value := range []string{
		strings.Repeat("s", 15), strings.Repeat("s", 256), strings.Repeat("é", 15),
		strings.Repeat("😀", 256), "  synthetic secret phrase  ", strings.Repeat(" ", 15),
	} {
		password := []byte(value)
		before := bytes.Clone(password)
		if err := ValidatePassword(password); err != nil || !bytes.Equal(password, before) {
			t.Fatal("valid literal password refused or changed", err)
		}
	}
	for _, password := range [][]byte{
		nil, []byte("synthetic"), []byte(strings.Repeat("é", 14)), []byte(strings.Repeat("s", 257)),
		[]byte(strings.Repeat("😀", 257)), append([]byte("synthetic-private"), 0xff),
	} {
		before := bytes.Clone(password)
		err := ValidatePassword(password)
		if !errors.Is(err, ErrInvalidPassword) || err.Error() != ErrInvalidPassword.Error() || !bytes.Equal(password, before) {
			t.Fatal("invalid password accepted, changed or disclosed", err)
		}
	}
}

func TestPasswordHashReferenceVectorAndMismatch(t *testing.T) {
	if argon2.Version != Argon2Version {
		t.Fatal("credential version differs from the cryptographic implementation")
	}
	password := []byte("synthetic secret phrase")
	before := bytes.Clone(password)
	random := strings.NewReader("0123456789abcdefunused")
	encoded, err := hashPassword(password, DefaultParameters(), random)
	if err != nil || encoded != syntheticPasswordHash || random.Len() != len("unused") || !bytes.Equal(password, before) {
		t.Fatal("hash differs from reference vector or input changed", err)
	}
	if err := ValidatePasswordHash(encoded); err != nil {
		t.Fatal("reference hash rejected", err)
	}
	for _, tc := range []struct {
		password string
		match    bool
	}{
		{"synthetic secret phrase", true},
		{"synthetic secret phrasa", false},
		{"Synthetic secret phrase", false},
		{" synthetic secret phrase ", false},
	} {
		match, err := VerifyPassword([]byte(tc.password), encoded)
		if err != nil || match != tc.match {
			t.Fatal("verification differs from reference/literal secret", err)
		}
	}
}

func TestPasswordHashFreshRandomSalt(t *testing.T) {
	password := []byte("synthetic random phrase")
	a, err := HashPassword(password, DefaultParameters())
	if err != nil {
		t.Fatal(err)
	}
	b, err := HashPassword(password, DefaultParameters())
	if err != nil || a == b || strings.Split(a, "$")[4] == strings.Split(b, "$")[4] {
		t.Fatal("two hashes reused their random salt", err)
	}
	if match, err := VerifyPassword(password, b); err != nil || !match {
		t.Fatal("randomly salted password did not verify", err)
	}
}

type forbiddenRandom struct{ t *testing.T }

func (r forbiddenRandom) Read([]byte) (int, error) {
	r.t.Fatal("invalid input attempted random salt generation")
	return 0, io.EOF
}

type failedRandom struct{}

func (failedRandom) Read([]byte) (int, error) {
	return 0, errors.New("synthetic-private-random-error")
}

func TestPasswordHashRejectsBeforeRandomnessAndNoPartialResult(t *testing.T) {
	for _, p := range []Parameters{{}, {MemoryKiB: 65536, Iterations: 3}, {MemoryKiB: 262145, Iterations: 3, Parallelism: 4}} {
		if encoded, err := hashPassword([]byte("synthetic secret phrase"), p, forbiddenRandom{t}); encoded != "" || !errors.Is(err, ErrInvalidParameters) {
			t.Fatal("invalid costs produced a credential", err)
		}
	}
	if encoded, err := hashPassword([]byte("synthetic"), DefaultParameters(), forbiddenRandom{t}); encoded != "" || !errors.Is(err, ErrInvalidPassword) {
		t.Fatal("invalid password produced a credential", err)
	}
	for _, random := range []io.Reader{nil, failedRandom{}, strings.NewReader("partial")} {
		encoded, err := hashPassword([]byte("synthetic secret phrase"), DefaultParameters(), random)
		if encoded != "" || !errors.Is(err, ErrRandom) || err.Error() != ErrRandom.Error() {
			t.Fatal("failed entropy produced partial credential or private diagnostic", err)
		}
	}
	if match, err := VerifyPassword([]byte("synthetic"), syntheticPasswordHash); match || !errors.Is(err, ErrInvalidPassword) {
		t.Fatal("invalid verification input accepted", err)
	}
}

func TestPasswordHashCodecStrictBoundsAndSafeErrors(t *testing.T) {
	// Validate maximal costs without actually allocating their Argon2 memory.
	max := strings.Replace(syntheticPasswordHash, "m=65536,t=3,p=4", "m=262144,t=6,p=4", 1)
	if err := ValidatePasswordHash(max); err != nil {
		t.Fatal("inclusive maximum profile rejected", err)
	}
	cases := []string{"", "synthetic-private-hash", strings.Repeat("s", 129), syntheticPasswordHash + "$", " " + syntheticPasswordHash, syntheticPasswordHash + "\n"}
	for _, replacement := range []string{
		"m=0,t=3,p=4", "m=65535,t=3,p=4", "m=262145,t=3,p=4", "m=4294967296,t=3,p=4",
		"m=65536,t=0,p=4", "m=65536,t=2,p=4", "m=65536,t=7,p=4", "m=65536,t=4294967296,p=4",
		"m=65536,t=3,p=0", "m=65536,t=3,p=5", "m=65536,t=3,p=256", "m=65536,t=3,p=3",
		"m=65537,t=3,p=4", "m=065536,t=3,p=4", "m=+65536,t=3,p=4", "m=-65536,t=3,p=4",
		"m=0x10000,t=3,p=4", "m=65536,t=03,p=4", "m=65536,t=3,p=04", "m=65536,t=3,p=4x",
		"t=3,m=65536,p=4", "m=65536,m=3,p=4", "m=65536,t=3,p=4,x=1", "m=65536, t=3,p=4",
	} {
		cases = append(cases, strings.Replace(syntheticPasswordHash, "m=65536,t=3,p=4", replacement, 1))
	}
	for _, tc := range []struct{ from, to string }{
		{"argon2id", "argon2i"}, {"argon2id", "Argon2id"}, {"v=19", "v=16"}, {"v=19", "v=019"},
		{"MDEyMzQ1Njc4OWFiY2RlZg", "MDEyMzQ1Njc4OWFiY2RlZg=="},
		{"MDEyMzQ1Njc4OWFiY2RlZg", "MDEyMzQ1Njc4OWFiY2RlZh"}, // nonzero padding bits
		{"MDEyMzQ1Njc4OWFiY2RlZg", "MDEyMzQ1Njc4OWFiY2RlZ"},
		{"MDEyMzQ1Njc4OWFiY2RlZg", "MDEyMzQ1Njc4OWFiY2Rl\r\n"},
		{"VmZ29U", "VmZ29V"}, // nonzero padding bits
		{"VmZ29U", "VmZ29"}, {"VmZ29U", "VmZ29U="}, {"VmZ29U", "VmZ29_"},
	} {
		cases = append(cases, strings.Replace(syntheticPasswordHash, tc.from, tc.to, 1))
	}
	for i, encoded := range cases {
		r, err := parsePasswordHash(encoded)
		if !errors.Is(err, ErrInvalidHash) || err.Error() != ErrInvalidHash.Error() || r != (passwordRecord{}) {
			t.Fatalf("case%d: invalid record returned partial data or private diagnostic", i)
		}
		if match, err := VerifyPassword([]byte("synthetic secret phrase"), encoded); match || !errors.Is(err, ErrInvalidHash) {
			t.Fatalf("case%d: unsafe record accepted for derivation", i)
		}
	}
}

func FuzzPasswordHashCodec(f *testing.F) {
	for _, seed := range []string{syntheticPasswordHash, "", "synthetic-private", "$argon2id$v=19$m=4294967296,t=3,p=4$$"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, encoded string) {
		r, err := parsePasswordHash(encoded)
		if err != nil {
			if err != ErrInvalidHash || r != (passwordRecord{}) {
				t.Fatal("unsafe rejection")
			}
		} else if r.params.Validate() != nil || len(encoded) > MaxEncodedHashBytes {
			t.Fatal("unbounded hash accepted")
		}
	})
}
