// Package nats provides a NATS JetStream broker for cross-service messaging.
//
// Blank-import to register:
//
//	import _ "github.com/godx-jp/godx-platform-framework/messaging/drivers/nats"
package nats

import (
	"context"
	"fmt"
	"strings"
	"sync"

	natsgo "github.com/nats-io/nats.go"

	mdriver "github.com/godx-jp/godx-platform-framework/messaging/driver"
)

func init() {
	mdriver.Register(mdriver.DriverNATS, New)
}

type broker struct {
	nc   *natsgo.Conn
	js   natsgo.JetStreamContext
	prefix string

	mu     sync.Mutex
	closed bool
	subs   []*natsgo.Subscription
}

func New(ctx context.Context, spec mdriver.Spec) (mdriver.Broker, error) {
	if spec.NATSURL == "" {
		return nil, fmt.Errorf("messaging/nats: NATSURL required")
	}
	nc, err := natsgo.Connect(spec.NATSURL, natsgo.Name("godx-messaging"))
	if err != nil {
		return nil, fmt.Errorf("messaging/nats: connect: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("messaging/nats: jetstream: %w", err)
	}
	prefix := spec.SubjectPrefix
	if prefix == "" {
		prefix = "godx."
	}
	b := &broker{nc: nc, js: js, prefix: prefix}
	if spec.StreamName != "" {
		if err := b.ensureStream(ctx, spec.StreamName); err != nil {
			_ = b.Close(ctx)
			return nil, err
		}
	}
	return b, nil
}

func (b *broker) ensureStream(_ context.Context, name string) error {
	subjects := []string{b.prefix + ">"}
	_, err := b.js.StreamInfo(name)
	if err == nil {
		return nil
	}
	_, err = b.js.AddStream(&natsgo.StreamConfig{
		Name:     name,
		Subjects: subjects,
		Storage:  natsgo.FileStorage,
	})
	if err != nil && !strings.Contains(err.Error(), "stream name already in use") {
		return fmt.Errorf("messaging/nats: add stream: %w", err)
	}
	return nil
}

func (b *broker) Name() string { return mdriver.DriverNATS }

func (b *broker) subject(name string) string {
	if strings.HasPrefix(name, b.prefix) {
		return name
	}
	return b.prefix + name
}

func (b *broker) Publish(_ context.Context, subject string, msg mdriver.Message) error {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return mdriver.ErrClosed
	}
	b.mu.Unlock()
	subject = b.subject(subject)
	nmsg := &natsgo.Msg{Subject: subject, Data: msg.Body, Header: natsgo.Header{}}
	for k, v := range msg.Headers {
		nmsg.Header.Set(k, v)
	}
	_, err := b.js.PublishMsg(nmsg)
	if err != nil {
		return err
	}
	return nil
}

func (b *broker) Subscribe(_ context.Context, subject string, handler mdriver.Handler) (mdriver.Subscription, error) {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil, mdriver.ErrClosed
	}
	b.mu.Unlock()
	subject = b.subject(subject)
	sub, err := b.js.Subscribe(subject, func(m *natsgo.Msg) {
		headers := map[string]string{}
		for k := range m.Header {
			headers[k] = m.Header.Get(k)
		}
		_ = handler(context.Background(), mdriver.Message{
			Subject: m.Subject,
			Body:    append([]byte(nil), m.Data...),
			Headers: headers,
		})
	})
	if err != nil {
		return nil, err
	}
	b.mu.Lock()
	b.subs = append(b.subs, sub)
	b.mu.Unlock()
	return &subscription{sub: sub}, nil
}

func (b *broker) Close(_ context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return nil
	}
	b.closed = true
	for _, sub := range b.subs {
		_ = sub.Unsubscribe()
	}
	b.subs = nil
	b.nc.Close()
	return nil
}

type subscription struct {
	sub *natsgo.Subscription
}

func (s *subscription) Cancel() error { return s.sub.Unsubscribe() }
