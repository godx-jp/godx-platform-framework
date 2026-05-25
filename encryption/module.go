package encryption

import (
	"context"
	"fmt"

	edriver "github.com/godx-jp/godx-platform-framework/encryption/driver"
	"github.com/godx-jp/godx-platform-framework/framework"
)

// StoreKey is the framework Store key under which the Encrypter is
// published.
const StoreKey = "godx.encryption.encrypter"

// Module is the default encryption module — reads ENCRYPTION_KEY +
// ENCRYPTION_DRIVER from env, builds the Encrypter, and publishes
// it into the App.
var Module framework.Module = envModule{}

type envModule struct{}

func (envModule) Name() string { return "encryption" }

func (envModule) Init(ctx context.Context, app *framework.App) error {
	cfg, err := LoadConfigFromEnv()
	if err != nil {
		return err
	}
	return initFromConfig(ctx, app, cfg)
}

// ModuleWithConfig returns a framework.Module that uses the supplied
// Config instead of loading from env.
func ModuleWithConfig(cfg Config) framework.Module {
	return &fixedModule{cfg: cfg}
}

type fixedModule struct{ cfg Config }

func (f *fixedModule) Name() string { return "encryption" }

func (f *fixedModule) Init(ctx context.Context, app *framework.App) error {
	return initFromConfig(ctx, app, f.cfg)
}

func initFromConfig(ctx context.Context, app *framework.App, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	if _, exists := app.Lookup(StoreKey); exists {
		return fmt.Errorf("encryption: Module already initialised (only one encryption.Module per App)")
	}
	cipher, err := edriver.New(ctx, edriver.Spec{Name: cfg.Driver})
	if err != nil {
		return err
	}
	enc := NewEncrypter(cipher)
	if err := enc.AddKey(cfg.PrimaryKeyID, cfg.PrimaryKey); err != nil {
		return err
	}
	for _, p := range cfg.Previous {
		if err := enc.AddKey(p.ID, p.Key); err != nil {
			return fmt.Errorf("encryption: load previous key %q: %w", p.ID, err)
		}
	}
	if err := enc.SetPrimary(cfg.PrimaryKeyID); err != nil {
		return err
	}
	app.Store(StoreKey, enc)
	return nil
}

// MustNew constructs an Encrypter for tests / scripts. keyEncoded is
// the same string format as ENCRYPTION_KEY (e.g.
// "base64:<RawStdBase64>"); aesgcm is used. Panics on bad input.
func MustNew(keyEncoded string) *Encrypter {
	key, err := ParseKey(keyEncoded)
	if err != nil {
		panic(err)
	}
	cipher, err := edriver.New(context.Background(), edriver.Spec{Name: edriver.DriverAESGCM})
	if err != nil {
		panic(err)
	}
	enc := NewEncrypter(cipher)
	if err := enc.AddKey("k1", key); err != nil {
		panic(err)
	}
	if err := enc.SetPrimary("k1"); err != nil {
		panic(err)
	}
	return enc
}
