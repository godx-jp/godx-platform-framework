package encryption

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"testing"

	edriver "github.com/godx-jp/godx-platform-framework/encryption/driver"
	"github.com/godx-jp/godx-platform-framework/framework"
)

func newKeyB64(t *testing.T) string {
	t.Helper()
	k := make([]byte, 32)
	if _, err := rand.Read(k); err != nil {
		t.Fatal(err)
	}
	return "base64:" + base64.StdEncoding.EncodeToString(k)
}

func TestModuleEnvDriven(t *testing.T) {
	t.Setenv(EnvKey, newKeyB64(t))
	t.Setenv(EnvDriver, "")
	app := framework.New("svc", "0.0.0").Use(Module)
	ctx := context.Background()
	if err := app.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()

	enc, err := FromApp(app)
	if err != nil {
		t.Fatalf("FromApp: %v", err)
	}
	if enc.CipherName() != edriver.DriverAESGCM {
		t.Fatalf("cipher: %q", enc.CipherName())
	}
}

func TestModulePreviousKeysLoaded(t *testing.T) {
	prev := newKeyB64(t)
	t.Setenv(EnvKey, newKeyB64(t))
	t.Setenv(EnvPreviousKeys, "old="+prev)
	app := framework.New("svc", "0.0.0").Use(Module)
	ctx := context.Background()
	if err := app.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()

	enc, _ := FromApp(app)
	ids := enc.KeyIDs()
	if len(ids) != 2 {
		t.Fatalf("KeyIDs: %v", ids)
	}
}

func TestModuleMissingKeyErrors(t *testing.T) {
	t.Setenv(EnvKey, "")
	app := framework.New("svc", "0.0.0").Use(Module)
	if err := app.Init(context.Background()); err == nil {
		t.Fatalf("missing ENCRYPTION_KEY should fail")
	}
}

func TestModuleValidateMalformedPreviousKey(t *testing.T) {
	t.Setenv(EnvKey, newKeyB64(t))
	t.Setenv(EnvPreviousKeys, "no-equals-sign")
	app := framework.New("svc", "0.0.0").Use(Module)
	if err := app.Init(context.Background()); err == nil {
		t.Fatalf("malformed previous key should fail")
	}
}

func TestModuleContextHelpers(t *testing.T) {
	enc := MustNew(newKeyB64(t))
	ctx := ContextWithEncrypter(context.Background(), enc)
	got, ok := FromContext(ctx)
	if !ok || got != enc {
		t.Fatalf("round trip")
	}
	if _, ok := FromContext(context.Background()); ok {
		t.Fatalf("plain ctx should miss")
	}
	if _, ok := FromContext(nil); ok {
		t.Fatalf("nil ctx should miss")
	}
}

func TestMustNewBuildsWorkingEncrypter(t *testing.T) {
	enc := MustNew(newKeyB64(t))
	ctx := context.Background()
	tok, err := enc.EncryptString(ctx, "x")
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	out, _ := enc.DecryptString(ctx, tok)
	if out != "x" {
		t.Fatalf("round trip %q", out)
	}
}
