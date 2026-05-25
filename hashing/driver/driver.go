// Package driver is the public contract for password-hashing
// implementations.
package driver

import "context"

// Hasher hashes a plaintext into a self-describing string and
// verifies candidates against that string. All implementations must
// be safe for concurrent use.
type Hasher interface {
	// Name returns the canonical driver name (bcrypt, argon2id,
	// scrypt). Used by registry, diagnostics, and Manager.
	Name() string

	// Make returns the encoded hash for plain.
	Make(ctx context.Context, plain string) (string, error)

	// Check reports whether plain matches the encoded hash. The hash
	// must have been produced by an implementation that understands
	// its encoding — usually the same Hasher, but anything that
	// understands $argon2id$ / $2[ay]$ / $scrypt$ tokens works.
	Check(ctx context.Context, plain, hash string) (bool, error)

	// NeedsRehash reports whether the encoded hash uses parameters
	// weaker than the Hasher's current configuration. Application
	// code should rehash after a successful Check when this is true.
	NeedsRehash(hash string) bool

	// Info returns the parameters embedded in the encoded hash. May
	// return ErrUnknownFormat for hashes the Hasher does not
	// recognise (callers should consult the registry to find the
	// right Hasher).
	Info(hash string) (Info, error)
}

// Info describes the parameters baked into one encoded hash.
type Info struct {
	// Algorithm is the canonical driver name that produced this hash.
	Algorithm string
	// Params holds the cost factors that matter for rehash decisions
	// (e.g. {"cost": "12"} for bcrypt; {"memory": "65536", "time":
	// "3", "threads": "2"} for argon2id).
	Params map[string]string
}

// Constructor builds a Hasher from a Spec. Each driver package
// exports one and registers it at init time.
type Constructor func(ctx context.Context, spec Spec) (Hasher, error)
