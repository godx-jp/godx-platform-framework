// Package apikey validates static API keys from a configurable header.
package apikey

import (
	"context"
	"crypto/subtle"
	"strings"
	"sync"

	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
)

const defaultHeader = "X-API-Key"

func init() {
	adriver.Register(adriver.DriverAPIKey, func(_ context.Context, spec adriver.Spec) (adriver.Guard, error) {
		header := strings.TrimSpace(spec.Header)
		if header == "" {
			header = defaultHeader
		}
		keys := make([]keyEntry, 0, len(spec.Keys))
		for _, entry := range spec.Keys {
			if entry.Secret == "" {
				continue
			}
			subject := entry.SubjectID
			if subject == "" {
				continue
			}
			keys = append(keys, keyEntry{
				subjectID:   subject,
				secret:      []byte(entry.Secret),
				actorKind:   entry.ActorKind,
				roles:       append([]string(nil), entry.Roles...),
				permissions: append([]string(nil), entry.Permissions...),
			})
		}
		return &guard{header: header, keys: keys}, nil
	})
}

type keyEntry struct {
	subjectID   string
	secret      []byte
	actorKind   adriver.ActorKind
	roles       []string
	permissions []string
}

type guard struct {
	header string
	keys   []keyEntry

	mu     sync.Mutex
	closed bool
}

func (g *guard) Name() string { return adriver.DriverAPIKey }

func (g *guard) Authenticate(_ context.Context, req *adriver.CredentialRequest) (*adriver.Principal, error) {
	if err := g.checkOpen(); err != nil {
		return nil, err
	}
	if req == nil || strings.TrimSpace(req.APIKey) == "" {
		return nil, adriver.ErrInvalidCredential
	}
	provided := []byte(strings.TrimSpace(req.APIKey))
	for _, k := range g.keys {
		if subtle.ConstantTimeCompare(provided, k.secret) == 1 {
			actor := k.actorKind
			if actor == "" {
				actor = adriver.ActorService
			}
			return &adriver.Principal{
				SubjectID:   k.subjectID,
				ActorKind:   actor,
				Roles:       append([]string(nil), k.roles...),
				Permissions: append([]string(nil), k.permissions...),
				Claims:      map[string]any{"api_key_subject": k.subjectID},
			}, nil
		}
	}
	return nil, adriver.ErrInvalidCredential
}

func (g *guard) Shutdown(context.Context) error {
	g.mu.Lock()
	g.closed = true
	g.mu.Unlock()
	return nil
}

func (g *guard) checkOpen() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.closed {
		return adriver.ErrClosed
	}
	return nil
}
