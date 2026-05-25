package driver

import "context"

// CredentialRequest carries extracted credentials for a guard.
type CredentialRequest struct {
	Guard  string
	Token  string
	APIKey string
	Extra  map[string]string
}

// Guard authenticates credentials and returns a Principal.
type Guard interface {
	// Name returns the canonical driver name.
	Name() string

	// Authenticate validates credentials and returns the authenticated principal.
	Authenticate(ctx context.Context, req *CredentialRequest) (*Principal, error)

	// Shutdown releases backend resources. Multiple calls must be safe.
	Shutdown(ctx context.Context) error
}

// Principal is the authenticated identity returned by a guard.
type Principal struct {
	SubjectID   string
	ActorKind   ActorKind
	Roles       []string
	Permissions []string
	Claims      map[string]any
	Guard       string
}

// ActorKind classifies who is acting.
type ActorKind string

const (
	ActorHuman      ActorKind = "human"
	ActorService    ActorKind = "service"
	ActorDevice     ActorKind = "device"
	ActorThirdParty ActorKind = "third_party"
)

// Constructor builds a Guard from Spec.
type Constructor func(ctx context.Context, spec Spec) (Guard, error)
