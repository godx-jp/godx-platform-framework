package auth

import (
	"fmt"
	"net/http"
	"strings"

	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
)

// CredentialResolver extracts credentials from an HTTP request for guards.
type CredentialResolver func(r *http.Request) (*adriver.CredentialRequest, error)

// BearerTokenResolver extracts an Authorization Bearer token.
func BearerTokenResolver() CredentialResolver {
	return func(r *http.Request) (*adriver.CredentialRequest, error) {
		if r == nil {
			return nil, adriver.ErrInvalidCredential
		}
		authz := strings.TrimSpace(r.Header.Get("Authorization"))
		const prefix = "Bearer "
		if !strings.HasPrefix(authz, prefix) {
			return nil, adriver.ErrInvalidCredential
		}
		token := strings.TrimSpace(strings.TrimPrefix(authz, prefix))
		if token == "" {
			return nil, adriver.ErrInvalidCredential
		}
		return &adriver.CredentialRequest{Token: token}, nil
	}
}

// APIKeyHeaderResolver extracts a value from the named header.
func APIKeyHeaderResolver(header string) CredentialResolver {
	if strings.TrimSpace(header) == "" {
		header = "X-API-Key"
	}
	return func(r *http.Request) (*adriver.CredentialRequest, error) {
		if r == nil {
			return nil, adriver.ErrInvalidCredential
		}
		key := strings.TrimSpace(r.Header.Get(header))
		if key == "" {
			return nil, adriver.ErrInvalidCredential
		}
		return &adriver.CredentialRequest{APIKey: key}, nil
	}
}

// ChainResolver tries resolvers in order and returns the first success.
func ChainResolver(resolvers ...CredentialResolver) CredentialResolver {
	return func(r *http.Request) (*adriver.CredentialRequest, error) {
		var lastErr error
		for _, resolve := range resolvers {
			if resolve == nil {
				continue
			}
			cred, err := resolve(r)
			if err == nil && cred != nil {
				return cred, nil
			}
			if err != nil {
				lastErr = err
			}
		}
		if lastErr != nil {
			return nil, lastErr
		}
		return nil, adriver.ErrInvalidCredential
	}
}

// GuardResolver binds a guard name to a resolver.
func GuardResolver(guardName string, resolve CredentialResolver) CredentialResolver {
	return func(r *http.Request) (*adriver.CredentialRequest, error) {
		cred, err := resolve(r)
		if err != nil {
			return nil, err
		}
		if cred != nil {
			cred.Guard = guardName
		}
		return cred, nil
	}
}

// ResolverForGuard picks a default resolver based on driver name.
func ResolverForGuard(driverName, header string) (CredentialResolver, error) {
	switch driverName {
	case adriver.DriverJWT:
		return BearerTokenResolver(), nil
	case adriver.DriverAPIKey:
		return APIKeyHeaderResolver(header), nil
	default:
		return nil, fmt.Errorf("auth: no default resolver for driver %q", driverName)
	}
}
