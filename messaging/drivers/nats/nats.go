// Package nats provides a NATS JetStream broker for cross-service messaging.
//
// Blank-import to register:
//
//	import _ "github.com/godx-jp/godx-platform-framework/messaging/drivers/nats"
package nats

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	natsgo "github.com/nats-io/nats.go"

	mdriver "github.com/godx-jp/godx-platform-framework/messaging/driver"
)

// defaultMaxPayloadBytes caps the size of an inbound message body the driver
// will copy and forward to a handler. SECURITY: without a ceiling a hostile or
// misbehaving publisher could push very large messages and force the consumer
// to allocate them unbounded (memory pressure / DoS). Override per connection
// with Spec.Extra["max_payload_bytes"]; a value <= 0 disables the check.
const defaultMaxPayloadBytes int64 = 1 << 20 // 1 MiB

func init() {
	mdriver.Register(mdriver.DriverNATS, New)
}

type broker struct {
	nc     *natsgo.Conn
	js     natsgo.JetStreamContext
	prefix string

	// maxPayload is the upper bound (bytes) on an inbound message body. A body
	// larger than this is dropped instead of being copied into a buffer.
	maxPayload int64

	mu     sync.Mutex
	closed bool
	subs   []*natsgo.Subscription
}

func New(ctx context.Context, spec mdriver.Spec) (mdriver.Broker, error) {
	if spec.NATSURL == "" {
		return nil, fmt.Errorf("messaging/nats: NATSURL required")
	}
	nc, err := natsgo.Connect(spec.NATSURL,
		natsgo.Name("godx-messaging"),
		natsgo.MaxReconnects(-1),
		natsgo.ReconnectWait(natsgo.DefaultReconnectWait),
		natsgo.DisconnectErrHandler(func(_ *natsgo.Conn, err error) {
			if err != nil {
				// logged by NATS client; broker stays alive for reconnect
			}
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("messaging/nats: connect: %w", err)
	}
	js, err := nc.JetStream()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("messaging/nats: jetstream: %w", err)
	}
	b := &broker{
		nc:         nc,
		js:         js,
		prefix:     spec.SubjectPrefix,
		maxPayload: extraInt64(spec.Extra, "max_payload_bytes", defaultMaxPayloadBytes),
	}
	if spec.StreamName != "" {
		if err := b.ensureStream(ctx, spec.StreamName, spec); err != nil {
			_ = b.Close(ctx)
			return nil, err
		}
	}
	return b, nil
}

func streamSubjects(prefix string) []string {
	if prefix == "" {
		return []string{">"}
	}
	return []string{prefix + ">"}
}

func (b *broker) ensureStream(_ context.Context, name string, spec mdriver.Spec) error {
	replicas := extraInt(spec.Extra, "stream_replicas", 1)
	_, err := b.js.StreamInfo(name)
	if err == nil {
		return nil
	}
	_, err = b.js.AddStream(&natsgo.StreamConfig{
		Name:     name,
		Subjects: streamSubjects(b.prefix),
		Storage:  natsgo.FileStorage,
		Replicas: replicas,
	})
	if err != nil && !strings.Contains(err.Error(), "stream name already in use") {
		return fmt.Errorf("messaging/nats: add stream: %w", err)
	}
	return nil
}

func extraInt(extra map[string]string, key string, def int) int {
	if extra == nil {
		return def
	}
	v := strings.TrimSpace(extra[key])
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		return def
	}
	return n
}

func extraInt64(extra map[string]string, key string, def int64) int64 {
	if extra == nil {
		return def
	}
	v := strings.TrimSpace(extra[key])
	if v == "" {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return def
	}
	return n
}

// payloadTooLarge reports whether a body of size bytes exceeds limit. A limit
// of <= 0 disables the check (unbounded). Kept as a tiny pure helper so the
// size guard can be unit-tested without a live NATS server.
func payloadTooLarge(size int, limit int64) bool {
	return limit > 0 && int64(size) > limit
}

func durableName(subject, override string) string {
	if override != "" {
		return override
	}
	return "godx_" + strings.NewReplacer(".", "_", "*", "all", ">", "all").Replace(subject)
}

func (b *broker) Name() string { return mdriver.DriverNATS }

func (b *broker) subject(name string) string {
	if b.prefix == "" {
		return name
	}
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

func (b *broker) Subscribe(ctx context.Context, subject string, handler mdriver.Handler) (mdriver.Subscription, error) {
	b.mu.Lock()
	if b.closed {
		b.mu.Unlock()
		return nil, mdriver.ErrClosed
	}
	b.mu.Unlock()
	subject = b.subject(subject)
	durable := durableName(subject, "")
	sub, err := b.js.Subscribe(subject, func(m *natsgo.Msg) {
		if ctx.Err() != nil {
			_ = m.Nak()
			return
		}
		// SECURITY: enforce an inbound payload ceiling before allocating. An
		// oversized body is never copied or forwarded; Term() tells JetStream
		// the message is permanently undeliverable so it is not redelivered
		// forever (a Nak would loop because the body cannot shrink).
		if payloadTooLarge(len(m.Data), b.maxPayload) {
			slog.Warn("messaging/nats: dropping oversized message",
				"subject", m.Subject,
				"size", len(m.Data),
				"max_payload_bytes", b.maxPayload,
			)
			_ = m.Term()
			return
		}
		headers := map[string]string{}
		for k := range m.Header {
			headers[k] = m.Header.Get(k)
		}
		err := handler(ctx, mdriver.Message{
			Subject: m.Subject,
			Body:    append([]byte(nil), m.Data...),
			Headers: headers,
		})
		if err != nil {
			_ = m.Nak()
			return
		}
		_ = m.Ack()
	}, natsgo.ManualAck(), natsgo.Durable(durable))
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
