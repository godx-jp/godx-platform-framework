// Package vault stores secrets in HashiCorp Vault KV-v2. Each
// secret is one KV entry whose payload has a single "value" field
// holding the raw bytes (base64-decoded when stored, base64-encoded
// on the wire because Vault KV-v2 stores strings).
//
//	store, err := vault.New(ctx, "https://vault:8200", "root", "secret", "myapp")
//
// The store is a heavy driver — import via:
//
//	_ "github.com/godx-jp/godx-platform-framework/secrets/drivers/vault"
package vault

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"path"
	"strings"
	"sync"

	vaultapi "github.com/hashicorp/vault/api"

	sdriver "github.com/godx-jp/godx-platform-framework/secrets/driver"
)

const defaultKVMount = "secret"

func init() {
	sdriver.Register(sdriver.DriverVault, func(ctx context.Context, spec sdriver.Spec) (sdriver.Store, error) {
		if spec.Address == "" {
			return nil, fmt.Errorf("secrets/vault: spec.Address is required")
		}
		mount := spec.KVMount
		if mount == "" {
			mount = defaultKVMount
		}
		return New(ctx, spec.Address, spec.Token, mount, spec.Prefix)
	})
}

// New constructs a Vault Store using the official client. The Vault
// SDK respects standard env vars (VAULT_ADDR, VAULT_TOKEN, etc.) but
// we forward explicit values when provided.
func New(ctx context.Context, address, token, kvMount, prefix string) (sdriver.Store, error) {
	cfg := vaultapi.DefaultConfig()
	if address != "" {
		cfg.Address = address
	}
	if err := cfg.ReadEnvironment(); err != nil {
		return nil, fmt.Errorf("secrets/vault: read env: %w", err)
	}
	cli, err := vaultapi.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("secrets/vault: client: %w", err)
	}
	if token != "" {
		cli.SetToken(token)
	}
	if kvMount == "" {
		kvMount = defaultKVMount
	}
	return &store{
		cli:    cli,
		kv:     cli.KVv2(kvMount),
		mount:  kvMount,
		prefix: strings.Trim(prefix, "/"),
	}, nil
}

type store struct {
	cli    *vaultapi.Client
	kv     *vaultapi.KVv2
	mount  string
	prefix string

	mu     sync.Mutex
	closed bool
}

func (s *store) Name() string { return sdriver.DriverVault }

func (s *store) keyPath(k string) string {
	k = strings.Trim(k, "/")
	if s.prefix != "" {
		return path.Join(s.prefix, k)
	}
	return k
}

func (s *store) Get(ctx context.Context, key string) ([]byte, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	if key == "" {
		return nil, sdriver.ErrNotFound
	}
	sec, err := s.kv.Get(ctx, s.keyPath(key))
	if err != nil {
		if isNotFound(err) {
			return nil, sdriver.ErrNotFound
		}
		return nil, fmt.Errorf("secrets/vault: get %s: %w", key, err)
	}
	if sec == nil || sec.Data == nil {
		return nil, sdriver.ErrNotFound
	}
	raw, ok := sec.Data["value"]
	if !ok {
		return nil, fmt.Errorf("secrets/vault: secret %s has no 'value' field", key)
	}
	enc, ok := raw.(string)
	if !ok {
		return nil, fmt.Errorf("secrets/vault: secret %s 'value' is not a string", key)
	}
	out, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		// Backwards-compat: also accept a plain string that wasn't
		// written by this driver.
		return []byte(enc), nil
	}
	return out, nil
}

func (s *store) Put(ctx context.Context, key string, value []byte) error {
	if err := s.checkOpen(); err != nil {
		return err
	}
	if key == "" {
		return fmt.Errorf("secrets/vault: empty key")
	}
	enc := base64.StdEncoding.EncodeToString(value)
	_, err := s.kv.Put(ctx, s.keyPath(key), map[string]any{"value": enc})
	if err != nil {
		return fmt.Errorf("secrets/vault: put %s: %w", key, err)
	}
	return nil
}

func (s *store) Forget(ctx context.Context, key string) error {
	if err := s.checkOpen(); err != nil {
		return err
	}
	if key == "" {
		return nil
	}
	if err := s.kv.Delete(ctx, s.keyPath(key)); err != nil {
		if isNotFound(err) {
			return nil
		}
		return fmt.Errorf("secrets/vault: delete %s: %w", key, err)
	}
	return nil
}

func (s *store) List(ctx context.Context) ([]string, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	// KV-v2 list goes through the metadata endpoint
	// /v1/<mount>/metadata/<prefix>?list=true.
	listPath := path.Join(s.mount, "metadata", s.prefix)
	sec, err := s.cli.Logical().ListWithContext(ctx, listPath)
	if err != nil {
		return nil, fmt.Errorf("secrets/vault: list: %w", err)
	}
	if sec == nil || sec.Data == nil {
		return nil, nil
	}
	rawKeys, ok := sec.Data["keys"].([]any)
	if !ok {
		return nil, nil
	}
	out := make([]string, 0, len(rawKeys))
	for _, k := range rawKeys {
		if s, ok := k.(string); ok {
			out = append(out, s)
		}
	}
	return out, nil
}

func (s *store) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	if s.cli != nil {
		// vaultapi.Client doesn't expose Close(); rely on http.Client GC.
		s.cli.ClearToken()
	}
	return nil
}

func (s *store) checkOpen() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return sdriver.ErrClosed
	}
	return nil
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	var rerr *vaultapi.ResponseError
	if errors.As(err, &rerr) && rerr.StatusCode == 404 {
		return true
	}
	return false
}
