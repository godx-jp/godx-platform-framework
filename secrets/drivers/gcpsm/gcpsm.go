// Package gcpsm stores secrets in Google Cloud Secret Manager.
// Versioning is left to GCP — Get always reads the latest enabled
// version of the named secret. Put creates a new version
// (auto-creating the secret resource on first write).
//
// The driver is heavy — import via:
//
//	_ "github.com/godx-jp/godx-platform-framework/secrets/drivers/gcpsm"
package gcpsm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	"cloud.google.com/go/secretmanager/apiv1/secretmanagerpb"
	"google.golang.org/api/iterator"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	sdriver "github.com/godx-jp/godx-platform-framework/secrets/driver"
)

func init() {
	sdriver.Register(sdriver.DriverGCPSM, func(ctx context.Context, spec sdriver.Spec) (sdriver.Store, error) {
		if spec.Project == "" {
			return nil, fmt.Errorf("secrets/gcpsm: spec.Project is required")
		}
		return New(ctx, spec.Project, spec.Prefix)
	})
}

type store struct {
	project string
	prefix  string

	cli *secretmanager.Client

	mu     sync.Mutex
	closed bool
}

// New constructs a GCP Secret Manager store using Application
// Default Credentials. project is the GCP project id. prefix (when
// non-empty) is prepended to every key so Get("db") on prefix
// "myapp" reads projects/<project>/secrets/myapp-db.
func New(ctx context.Context, project, prefix string) (sdriver.Store, error) {
	c, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("secrets/gcpsm: client: %w", err)
	}
	return &store{
		project: project,
		prefix:  strings.Trim(prefix, "-/"),
		cli:     c,
	}, nil
}

// secretID resolves a logical key to a GCP secret-resource id —
// only letters, digits, dashes and underscores are valid in GCP
// secret ids, so we normalise.
func (s *store) secretID(key string) string {
	clean := strings.NewReplacer("/", "-", ".", "-", " ", "-").Replace(key)
	if s.prefix != "" {
		return s.prefix + "-" + clean
	}
	return clean
}

func (s *store) parentPath() string      { return "projects/" + s.project }
func (s *store) secretPath(k string) string {
	return s.parentPath() + "/secrets/" + s.secretID(k)
}
func (s *store) versionPath(k string) string { return s.secretPath(k) + "/versions/latest" }

func (s *store) Name() string { return sdriver.DriverGCPSM }

func (s *store) Get(ctx context.Context, key string) ([]byte, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	if key == "" {
		return nil, sdriver.ErrNotFound
	}
	resp, err := s.cli.AccessSecretVersion(ctx, &secretmanagerpb.AccessSecretVersionRequest{
		Name: s.versionPath(key),
	})
	if err != nil {
		if isNotFound(err) {
			return nil, sdriver.ErrNotFound
		}
		return nil, fmt.Errorf("secrets/gcpsm: access %s: %w", key, err)
	}
	return resp.GetPayload().GetData(), nil
}

func (s *store) Put(ctx context.Context, key string, value []byte) error {
	if err := s.checkOpen(); err != nil {
		return err
	}
	if key == "" {
		return fmt.Errorf("secrets/gcpsm: empty key")
	}
	_, err := s.cli.CreateSecret(ctx, &secretmanagerpb.CreateSecretRequest{
		Parent:   s.parentPath(),
		SecretId: s.secretID(key),
		Secret: &secretmanagerpb.Secret{
			Replication: &secretmanagerpb.Replication{
				Replication: &secretmanagerpb.Replication_Automatic_{
					Automatic: &secretmanagerpb.Replication_Automatic{},
				},
			},
		},
	})
	if err != nil && !isAlreadyExists(err) {
		return fmt.Errorf("secrets/gcpsm: create %s: %w", key, err)
	}
	_, err = s.cli.AddSecretVersion(ctx, &secretmanagerpb.AddSecretVersionRequest{
		Parent:  s.secretPath(key),
		Payload: &secretmanagerpb.SecretPayload{Data: value},
	})
	if err != nil {
		return fmt.Errorf("secrets/gcpsm: add version %s: %w", key, err)
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
	err := s.cli.DeleteSecret(ctx, &secretmanagerpb.DeleteSecretRequest{Name: s.secretPath(key)})
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return fmt.Errorf("secrets/gcpsm: delete %s: %w", key, err)
	}
	return nil
}

func (s *store) List(ctx context.Context) ([]string, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	it := s.cli.ListSecrets(ctx, &secretmanagerpb.ListSecretsRequest{Parent: s.parentPath()})
	prefix := s.prefix
	if prefix != "" {
		prefix += "-"
	}
	var out []string
	for {
		sec, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("secrets/gcpsm: list: %w", err)
		}
		name := sec.GetName()
		const sep = "/secrets/"
		if idx := strings.Index(name, sep); idx >= 0 {
			id := name[idx+len(sep):]
			if prefix != "" {
				if !strings.HasPrefix(id, prefix) {
					continue
				}
				id = strings.TrimPrefix(id, prefix)
			}
			out = append(out, id)
		}
	}
	return out, nil
}

func (s *store) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	s.closed = true
	if s.cli != nil {
		return s.cli.Close()
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
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	return st.Code() == codes.NotFound
}

func isAlreadyExists(err error) bool {
	st, ok := status.FromError(err)
	if !ok {
		return false
	}
	return st.Code() == codes.AlreadyExists
}
