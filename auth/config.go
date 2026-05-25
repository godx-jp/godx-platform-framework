package auth

import (
	"fmt"
	"os"
	"strings"

	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
)

const (
	EnvDefault = "AUTH_DEFAULT"
	EnvGuards  = "AUTH_GUARDS"

	envGuardDriver = "AUTH_GUARD_%s_DRIVER"
	envGuardJWKS   = "AUTH_GUARD_%s_JWKS_URL"
	envGuardIssuer = "AUTH_GUARD_%s_ISSUER"
	envGuardAud    = "AUTH_GUARD_%s_AUDIENCE"
	envGuardRoles  = "AUTH_GUARD_%s_ROLES_CLAIM"
	envGuardPerms  = "AUTH_GUARD_%s_PERMISSIONS_CLAIM"
	envGuardSub    = "AUTH_GUARD_%s_SUBJECT_CLAIM"
	envGuardActor  = "AUTH_GUARD_%s_ACTOR_KIND_CLAIM"
	envGuardHeader = "AUTH_GUARD_%s_HEADER"
	envGuardKeys   = "AUTH_GUARD_%s_KEYS"
	envGuardIntro  = "AUTH_GUARD_%s_INTROSPECT_URL"

	envGuardKeyRoles = "AUTH_GUARD_%s_KEY_%s_ROLES"
	envGuardKeyPerms = "AUTH_GUARD_%s_KEY_%s_PERMISSIONS"
	envGuardKeyActor = "AUTH_GUARD_%s_KEY_%s_ACTOR_KIND"
)

type GuardConfig struct {
	Driver string
	Spec   adriver.Spec
}

type Config struct {
	Default string
	Guards  map[string]GuardConfig
}

func LoadConfigFromEnv() Config {
	def := strings.TrimSpace(os.Getenv(EnvDefault))
	if def == "" {
		def = adriver.DriverAPIKey
	}
	names := splitCSV(os.Getenv(EnvGuards))
	if len(names) == 0 {
		names = []string{def}
	}

	guards := make(map[string]GuardConfig, len(names))
	for _, name := range names {
		driver := inferDriver(name)
		guards[name] = GuardConfig{
			Driver: driver,
			Spec:   loadSpec(name, driver),
		}
	}
	return Config{Default: def, Guards: guards}
}

func inferDriver(name string) string {
	key := fmt.Sprintf(envGuardDriver, envKey(name))
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	switch strings.ToLower(name) {
	case adriver.DriverJWT, adriver.DriverAPIKey, adriver.DriverIntrospect:
		return strings.ToLower(name)
	default:
		return name
	}
}

func loadSpec(name, driver string) adriver.Spec {
	spec := adriver.Spec{
		Name:             driver,
		RolesClaim:       "roles",
		PermissionsClaim: "permissions",
		SubjectClaim:     "sub",
		Header:           "X-API-Key",
	}
	spec.JWKSURL = os.Getenv(fmt.Sprintf(envGuardJWKS, envKey(name)))
	spec.Issuer = os.Getenv(fmt.Sprintf(envGuardIssuer, envKey(name)))
	spec.Audience = os.Getenv(fmt.Sprintf(envGuardAud, envKey(name)))
	if v := os.Getenv(fmt.Sprintf(envGuardRoles, envKey(name))); v != "" {
		spec.RolesClaim = v
	}
	if v := os.Getenv(fmt.Sprintf(envGuardPerms, envKey(name))); v != "" {
		spec.PermissionsClaim = v
	}
	if v := os.Getenv(fmt.Sprintf(envGuardSub, envKey(name))); v != "" {
		spec.SubjectClaim = v
	}
	spec.ActorKindClaim = os.Getenv(fmt.Sprintf(envGuardActor, envKey(name)))
	if v := os.Getenv(fmt.Sprintf(envGuardHeader, envKey(name))); v != "" {
		spec.Header = v
	}
	spec.IntrospectURL = os.Getenv(fmt.Sprintf(envGuardIntro, envKey(name)))
	if keysCSV := os.Getenv(fmt.Sprintf(envGuardKeys, envKey(name))); keysCSV != "" {
		spec.Keys = parseAPIKeys(name, keysCSV)
	}
	return spec
}

func parseAPIKeys(guardName, csv string) map[string]adriver.APIKeyEntry {
	out := map[string]adriver.APIKeyEntry{}
	for _, part := range splitCSV(csv) {
		subject, secret, ok := strings.Cut(part, ":")
		if !ok || strings.TrimSpace(subject) == "" || strings.TrimSpace(secret) == "" {
			continue
		}
		subject = strings.TrimSpace(subject)
		secret = strings.TrimSpace(secret)
		entry := adriver.APIKeyEntry{
			SubjectID: subject,
			Secret:    secret,
		}
		keyEnv := envKey(subject)
		if v := os.Getenv(fmt.Sprintf(envGuardKeyRoles, envKey(guardName), keyEnv)); v != "" {
			entry.Roles = splitCSV(v)
		}
		if v := os.Getenv(fmt.Sprintf(envGuardKeyPerms, envKey(guardName), keyEnv)); v != "" {
			entry.Permissions = splitCSV(v)
		}
		if v := os.Getenv(fmt.Sprintf(envGuardKeyActor, envKey(guardName), keyEnv)); v != "" {
			entry.ActorKind = adriver.ActorKind(v)
		}
		out[subject] = entry
	}
	return out
}

func (c Config) Validate() error {
	if c.Default == "" {
		return fmt.Errorf("auth: default guard name required")
	}
	if len(c.Guards) == 0 {
		return fmt.Errorf("auth: no guards configured")
	}
	if _, ok := c.Guards[c.Default]; !ok {
		return fmt.Errorf("auth: default %q not in Guards", c.Default)
	}
	for name, gc := range c.Guards {
		if strings.TrimSpace(gc.Driver) == "" {
			return fmt.Errorf("auth: guard %q: driver is required", name)
		}
	}
	return nil
}

func envKey(name string) string {
	return strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
