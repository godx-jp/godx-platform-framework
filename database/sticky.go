package database

import (
	"context"
)

type stickyKey struct{}

// MarkWritten marks ctx so subsequent Read() calls use the write connection.
func MarkWritten(ctx context.Context) context.Context {
	return context.WithValue(ctx, stickyKey{}, true)
}

// WasWritten reports whether MarkWritten was called on ctx or an ancestor.
func WasWritten(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	w, ok := ctx.Value(stickyKey{}).(bool)
	return ok && w
}
