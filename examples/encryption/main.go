// Run with `go run ./examples/encryption` from the repo root.
//
// AES-GCM (default), key generated for the demo:
//
//	ENCRYPTION_KEY=base64:$(openssl rand -base64 32) \
//	go run ./examples/encryption
//
// ChaCha20-Poly1305:
//
//	ENCRYPTION_DRIVER=chacha20poly1305 \
//	ENCRYPTION_KEY=base64:$(openssl rand -base64 32) \
//	go run ./examples/encryption
//
// With a previous-key entry so old ciphertext stays decryptable
// through a rotation:
//
//	ENCRYPTION_KEY=base64:$NEWKEY \
//	ENCRYPTION_PREVIOUS_KEYS="legacy=base64:$OLDKEY" \
//	ENCRYPTION_PRIMARY_KEY_ID=current \
//	go run ./examples/encryption
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log"
	"os"

	"github.com/godx-jp/godx-platform-framework/encryption"
	"github.com/godx-jp/godx-platform-framework/framework"
)

func main() {
	if os.Getenv(encryption.EnvKey) == "" {
		k := make([]byte, 32)
		if _, err := rand.Read(k); err != nil {
			log.Fatal(err)
		}
		os.Setenv(encryption.EnvKey, "base64:"+base64.StdEncoding.EncodeToString(k))
		fmt.Println("[example] generated demo key for this run")
	}

	ctx := context.Background()
	app := framework.New("encryption-example", "0.0.0").Use(encryption.Module)
	if err := app.Init(ctx); err != nil {
		log.Fatalf("init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()

	enc, err := encryption.FromApp(app)
	if err != nil {
		log.Fatalf("encryption not wired: %v", err)
	}

	fmt.Printf("cipher: %s\n", enc.CipherName())
	fmt.Printf("primary key id: %s\n", enc.PrimaryKeyID())

	tok, err := enc.EncryptString(ctx, "the rain in spain falls mainly on the plain")
	if err != nil {
		log.Fatalf("Encrypt: %v", err)
	}
	fmt.Printf("token:   %s\n", tok)

	plain, _ := enc.DecryptString(ctx, tok)
	fmt.Printf("plain:   %s\n", plain)

	id, _ := encryption.KeyIDOf(tok)
	fmt.Printf("token key id: %s\n", id)
}
