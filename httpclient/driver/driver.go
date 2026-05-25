package driver

import (
	"context"
	"net/http"
)

// Client is one HTTP backend implementation.
type Client interface {
	Name() string
	Do(ctx context.Context, req *http.Request) (*http.Response, error)
	Shutdown(ctx context.Context) error
}

// Constructor builds a Client from Spec.
type Constructor func(ctx context.Context, spec Spec) (Client, error)
