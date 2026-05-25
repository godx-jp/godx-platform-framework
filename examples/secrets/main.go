// Run with `go run ./examples/secrets` from the repo root.
//
// The default-prefix env-vars driver looks up secrets under
// SECRETS_<KEY> (slashes / dots / dashes become underscores, then
// upper-case). Try:
//
//	SECRETS_DB_PASSWORD=hunter2 SECRETS_AUTH_TOKEN=abc123 \
//	  go run ./examples/secrets
//
// To exercise the file driver against a directory of K8s-style
// secret mounts:
//
//	mkdir -p /tmp/example-secrets
//	echo -n hunter2 > /tmp/example-secrets/db-password
//	SECRETS_DEFAULT=file SECRETS_STORES=file \
//	  SECRETS_FILE_PATH=/tmp/example-secrets \
//	  go run ./examples/secrets
package main

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/godx-jp/godx-platform-framework/framework"
	"github.com/godx-jp/godx-platform-framework/secrets"
	sdriver "github.com/godx-jp/godx-platform-framework/secrets/driver"
)

func main() {
	ctx := context.Background()
	app := framework.New("secrets-example", "0.0.0").Use(secrets.Module)
	if err := app.Init(ctx); err != nil {
		log.Fatalf("init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()

	mgr, err := secrets.FromApp(app)
	if err != nil {
		log.Fatalf("secrets not wired: %v", err)
	}
	fmt.Printf("default store: %s\n", mgr.Default().Name())

	for _, key := range []string{"db/password", "auth/token"} {
		v, err := mgr.GetString(ctx, key)
		switch {
		case errors.Is(err, sdriver.ErrNotFound):
			fmt.Printf("  %-15s (missing)\n", key)
		case err != nil:
			fmt.Printf("  %-15s ERROR: %v\n", key, err)
		default:
			fmt.Printf("  %-15s = %s\n", key, redact(v))
		}
	}

	// Demonstrate Put on writable stores (no-op on env driver — it
	// returns ErrReadOnly, which we surface so the user understands).
	if err := mgr.PutString(ctx, "session/key", "ephemeral"); err != nil {
		fmt.Printf("PutString: %v (expected on read-only stores)\n", err)
	}
}

func redact(s string) string {
	if len(s) <= 2 {
		return "**"
	}
	return s[:1] + "***" + s[len(s)-1:]
}
