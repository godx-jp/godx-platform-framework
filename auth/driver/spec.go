package driver

import "time"

const (
	DriverJWT        = "jwt"
	DriverHMAC       = "hmac"
	DriverAPIKey     = "apikey"
	DriverIntrospect = "introspect"
)

// APIKeyEntry configures one static API key.
type APIKeyEntry struct {
	SubjectID   string
	Secret      string
	ActorKind   ActorKind
	Roles       []string
	Permissions []string
}

// Spec configures an auth guard driver.
type Spec struct {
	Name string

	// ── jwt driver ───────────────────────────────────────────────
	JWKSURL          string
	Issuer           string
	Audience         string
	RolesClaim       string
	PermissionsClaim string
	SubjectClaim     string
	ActorKindClaim   string
	Secret           string
	Alg              string

	// ── apikey driver ────────────────────────────────────────────
	Header string
	Keys   map[string]APIKeyEntry

	// ── introspect driver ────────────────────────────────────────
	IntrospectURL string

	// ── hmac driver (symmetric HS256 JWT) ────────────────────────
	LeewaySeconds int64
	MaxTokenTTL   time.Duration

	Extra map[string]string
}
