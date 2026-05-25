// Package awssm stores secrets in AWS Secrets Manager. Each logical
// key maps to a secret resource whose SecretBinary holds the raw
// bytes. Put creates a new secret on first write and updates it on
// subsequent writes.
//
// The driver is heavy — import via:
//
//	_ "github.com/godx-jp/godx-platform-framework/secrets/drivers/awssm"
package awssm

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	smtypes "github.com/aws/aws-sdk-go-v2/service/secretsmanager/types"

	sdriver "github.com/godx-jp/godx-platform-framework/secrets/driver"
)

func init() {
	sdriver.Register(sdriver.DriverAWSSM, func(ctx context.Context, spec sdriver.Spec) (sdriver.Store, error) {
		return New(ctx, spec.Region, spec.Prefix)
	})
}

// New constructs an AWS Secrets Manager store. region overrides
// AWS_REGION when non-empty. prefix is prepended to every secret
// name so Get("db") on prefix "myapp/" reads "myapp/db".
func New(ctx context.Context, region, prefix string) (sdriver.Store, error) {
	var loadOpts []func(*awsconfig.LoadOptions) error
	if region != "" {
		loadOpts = append(loadOpts, awsconfig.WithRegion(region))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("secrets/awssm: load aws config: %w", err)
	}
	return &store{
		cli:    secretsmanager.NewFromConfig(cfg),
		prefix: strings.TrimRight(prefix, "/"),
	}, nil
}

type store struct {
	cli    *secretsmanager.Client
	prefix string

	mu     sync.Mutex
	closed bool
}

func (s *store) Name() string { return sdriver.DriverAWSSM }

func (s *store) secretName(key string) string {
	clean := strings.TrimLeft(key, "/")
	if s.prefix != "" {
		return s.prefix + "/" + clean
	}
	return clean
}

func (s *store) Get(ctx context.Context, key string) ([]byte, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	if key == "" {
		return nil, sdriver.ErrNotFound
	}
	resp, err := s.cli.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(s.secretName(key)),
	})
	if err != nil {
		var nf *smtypes.ResourceNotFoundException
		if errors.As(err, &nf) {
			return nil, sdriver.ErrNotFound
		}
		return nil, fmt.Errorf("secrets/awssm: get %s: %w", key, err)
	}
	if resp.SecretBinary != nil {
		return resp.SecretBinary, nil
	}
	if resp.SecretString != nil {
		return []byte(*resp.SecretString), nil
	}
	return nil, sdriver.ErrNotFound
}

func (s *store) Put(ctx context.Context, key string, value []byte) error {
	if err := s.checkOpen(); err != nil {
		return err
	}
	if key == "" {
		return fmt.Errorf("secrets/awssm: empty key")
	}
	name := s.secretName(key)
	_, err := s.cli.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(name),
		SecretBinary: value,
	})
	if err == nil {
		return nil
	}
	var nf *smtypes.ResourceNotFoundException
	if !errors.As(err, &nf) {
		return fmt.Errorf("secrets/awssm: put %s: %w", key, err)
	}
	_, err = s.cli.CreateSecret(ctx, &secretsmanager.CreateSecretInput{
		Name:         aws.String(name),
		SecretBinary: value,
	})
	if err != nil {
		return fmt.Errorf("secrets/awssm: create %s: %w", key, err)
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
	_, err := s.cli.DeleteSecret(ctx, &secretsmanager.DeleteSecretInput{
		SecretId:                   aws.String(s.secretName(key)),
		ForceDeleteWithoutRecovery: aws.Bool(true),
	})
	if err != nil {
		var nf *smtypes.ResourceNotFoundException
		if errors.As(err, &nf) {
			return nil
		}
		return fmt.Errorf("secrets/awssm: delete %s: %w", key, err)
	}
	return nil
}

func (s *store) List(ctx context.Context) ([]string, error) {
	if err := s.checkOpen(); err != nil {
		return nil, err
	}
	var (
		out  []string
		next *string
	)
	for {
		resp, err := s.cli.ListSecrets(ctx, &secretsmanager.ListSecretsInput{
			NextToken: next,
		})
		if err != nil {
			return nil, fmt.Errorf("secrets/awssm: list: %w", err)
		}
		for _, e := range resp.SecretList {
			if e.Name == nil {
				continue
			}
			name := *e.Name
			if s.prefix != "" {
				p := s.prefix + "/"
				if !strings.HasPrefix(name, p) {
					continue
				}
				name = strings.TrimPrefix(name, p)
			}
			out = append(out, name)
		}
		if resp.NextToken == nil || *resp.NextToken == "" {
			return out, nil
		}
		next = resp.NextToken
	}
}

func (s *store) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	s.closed = true
	s.mu.Unlock()
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
