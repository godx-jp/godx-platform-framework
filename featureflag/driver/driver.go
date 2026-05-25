package driver

import (
	"context"
	"errors"
)

var (
	ErrClosed         = errors.New("featureflag: provider is closed")
	ErrNotConfigured  = errors.New("featureflag: provider not configured")
	ErrUnknownDriver  = errors.New("featureflag: unknown driver")
)

// Canonical driver names.
const (
	DriverConfig       = "config"
	DriverOpenFeature  = "openfeature"
	DriverLaunchDarkly = "launchdarkly"
	DriverUnleash      = "unleash"
	DriverFlagsmith    = "flagsmith"
)

// Provider evaluates one feature flag for a user and attribute map.
type Provider interface {
	Name() string
	Enabled(ctx context.Context, flag, user string, attrs map[string]any) (bool, error)
	Shutdown(ctx context.Context) error
}

// Constructor builds a Provider from Spec.
type Constructor func(ctx context.Context, spec Spec) (Provider, error)
