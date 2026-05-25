// Package hashing implements password hashing — Laravel's Hash
// facade reimagined for Go. A Hasher driver hashes a plaintext into
// a self-describing string and verifies a candidate against that
// string; the driver also reports whether the stored hash should be
// rehashed (work factor upgrade).
//
//	h := hashing.MustDefault()
//	enc, _ := h.Make("hunter2")
//	ok,  _ := h.Check("hunter2", enc)
//	if h.NeedsRehash(enc) { enc, _ = h.Make("hunter2") }
//
// Driver selection is one env var (`HASHING_DRIVER`); the Manager
// wires named hashers à la cache.Manager so apps can run bcrypt for
// legacy users and argon2id for new signups side by side.
//
// Laravel mapping:
//
//	Laravel                            | Framework
//	-----------------------------------|----------------------------
//	Hash::make($plain)                  | h.Make(plain)
//	Hash::check($plain, $hash)          | h.Check(plain, hash)
//	Hash::needsRehash($hash)           | h.NeedsRehash(hash)
//	Hash::info($hash)                  | h.Info(hash)  (driver, params)
//	config/hashing.php driver swap     | HASHING_DRIVER env var
package hashing
