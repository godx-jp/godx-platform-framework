// Package ses sends mail via Amazon SES v2. Heavy — import via:
//
//	_ "github.com/godx-jp/godx-platform-framework/mail/drivers/ses"
package ses

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sesv2"
	"github.com/aws/aws-sdk-go-v2/service/sesv2/types"

	mdriver "github.com/godx-jp/godx-platform-framework/mail/driver"
)

func init() {
	mdriver.Register(mdriver.DriverSES, func(ctx context.Context, spec mdriver.Spec) (mdriver.Transport, error) {
		return New(ctx, spec)
	})
}

type transport struct {
	cli  *sesv2.Client
	from string

	mu     sync.Mutex
	closed bool
}

// New constructs an SES Transport. region overrides AWS_REGION when set.
func New(ctx context.Context, spec mdriver.Spec) (mdriver.Transport, error) {
	var loadOpts []func(*awsconfig.LoadOptions) error
	if spec.Region != "" {
		loadOpts = append(loadOpts, awsconfig.WithRegion(spec.Region))
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, loadOpts...)
	if err != nil {
		return nil, fmt.Errorf("mail/ses: load aws config: %w", err)
	}
	return &transport{
		cli:  sesv2.NewFromConfig(cfg),
		from: spec.From,
	}, nil
}

func (t *transport) Name() string { return mdriver.DriverSES }

func (t *transport) Send(ctx context.Context, msg mdriver.Message) error {
	if err := t.checkOpen(); err != nil {
		return err
	}
	if len(msg.To) == 0 {
		return fmt.Errorf("mail/ses: at least one recipient is required")
	}
	from := msg.From
	if from == "" {
		from = t.from
	}
	if from == "" {
		return fmt.Errorf("mail/ses: from address is required")
	}
	body := msg.Body
	contentType := "text/plain"
	if msg.HTMLBody != "" {
		body = msg.HTMLBody
		contentType = "text/html"
	}
	_, err := t.cli.SendEmail(ctx, &sesv2.SendEmailInput{
		FromEmailAddress: aws.String(from),
		Destination: &types.Destination{
			ToAddresses: msg.To,
		},
		Content: &types.EmailContent{
			Simple: &types.Message{
				Subject: &types.Content{Data: aws.String(msg.Subject)},
				Body: &types.Body{
					Text: &types.Content{Data: aws.String(body), Charset: aws.String("UTF-8")},
				},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("mail/ses: send: %w", err)
	}
	_ = contentType // reserved for future HTML branch
	return nil
}

func (t *transport) Shutdown(context.Context) error {
	t.mu.Lock()
	t.closed = true
	t.mu.Unlock()
	return nil
}

func (t *transport) checkOpen() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.closed {
		return mdriver.ErrClosed
	}
	return nil
}

// ValidateFrom ensures a from address is configured (for tests).
func ValidateFrom(from string) error {
	if strings.TrimSpace(from) == "" {
		return fmt.Errorf("mail/ses: from address is required for production sends")
	}
	return nil
}
