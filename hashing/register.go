package hashing

// All built-in hashing drivers are light (pure CPU / stdlib + x/crypto)
// so we auto-register them via blank imports.
import (
	_ "github.com/godx-jp/godx-platform-framework/hashing/drivers/argon2id"
	_ "github.com/godx-jp/godx-platform-framework/hashing/drivers/bcrypt"
	_ "github.com/godx-jp/godx-platform-framework/hashing/drivers/scrypt"
)
