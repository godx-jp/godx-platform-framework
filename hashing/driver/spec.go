package driver

// Spec is the uniform input to every hasher constructor.
type Spec struct {
	// Name is the driver name (bcrypt, argon2id, scrypt).
	Name string

	// ── bcrypt ──────────────────────────────────────────────────
	// BcryptCost is the cost factor (4..31). Defaults to 12.
	BcryptCost int

	// ── argon2id ────────────────────────────────────────────────
	// Argon2Time is the number of iterations. Defaults to 3.
	Argon2Time uint32
	// Argon2Memory is the memory cost in KiB. Defaults to 65536 (64 MiB).
	Argon2Memory uint32
	// Argon2Threads is the parallelism. Defaults to 2.
	Argon2Threads uint8
	// Argon2KeyLength is the output length in bytes. Defaults to 32.
	Argon2KeyLength uint32
	// Argon2SaltLength is the random salt length in bytes. Defaults to 16.
	Argon2SaltLength uint32

	// ── scrypt ──────────────────────────────────────────────────
	// ScryptN is the CPU/memory cost (must be a power of 2). Defaults to 32768.
	ScryptN int
	// ScryptR is the block size. Defaults to 8.
	ScryptR int
	// ScryptP is the parallelism. Defaults to 1.
	ScryptP int
	// ScryptKeyLength is the output length. Defaults to 32.
	ScryptKeyLength int
	// ScryptSaltLength is the random salt length. Defaults to 16.
	ScryptSaltLength int

	// Extra carries driver-specific extension config.
	Extra map[string]string
}
