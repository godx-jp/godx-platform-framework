package driver

import "context"

type Message struct {
	Subject string
	Body    []byte
	Headers map[string]string
}

type Handler func(ctx context.Context, msg Message) error

type Subscription interface {
	Cancel() error
}

type Broker interface {
	Name() string
	Publish(ctx context.Context, subject string, msg Message) error
	Subscribe(ctx context.Context, subject string, handler Handler) (Subscription, error)
	Close(ctx context.Context) error
}

type Constructor func(ctx context.Context, spec Spec) (Broker, error)
