package driver

import "github.com/godx-jp/godx-platform-framework/config"

// Spec carries per-driver configuration knobs.
type Spec struct {
	Name string

	// Config driver — prefix for flag keys (default "flags").
	Prefix string
	Repo   *config.Repository

	// Heavy drivers — connection endpoints / SDK keys (stubs validate presence).
	SDKKey   string
	Endpoint string
	Project  string
	AppName  string
}
