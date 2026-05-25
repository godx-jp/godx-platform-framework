package encryption

import (
	"context"
	"fmt"

	"github.com/godx-jp/godx-platform-framework/framework"
)

type contextKey struct{}

// ContextWithEncrypter returns a derived context carrying enc.
func ContextWithEncrypter(ctx context.Context, enc *Encrypter) context.Context {
	return context.WithValue(ctx, contextKey{}, enc)
}

// FromContext retrieves the Encrypter attached to ctx.
func FromContext(ctx context.Context) (*Encrypter, bool) {
	if ctx == nil {
		return nil, false
	}
	enc, ok := ctx.Value(contextKey{}).(*Encrypter)
	return enc, ok
}

// FromApp returns the Encrypter published by encryption.Module on app.
func FromApp(app *framework.App) (*Encrypter, error) {
	v, ok := app.Lookup(StoreKey)
	if !ok {
		return nil, fmt.Errorf("encryption: Module has not been initialised on this App")
	}
	enc, ok := v.(*Encrypter)
	if !ok {
		return nil, fmt.Errorf("encryption: %s framework Store entry is not an *Encrypter (%T)", StoreKey, v)
	}
	return enc, nil
}
