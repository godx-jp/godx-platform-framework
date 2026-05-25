package stdlib

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	hdriver "github.com/godx-jp/godx-platform-framework/httpclient/driver"
	"github.com/godx-jp/godx-platform-framework/httpclient/middleware"
)

func init() {
	hdriver.Register(hdriver.DriverStdlib, func(ctx context.Context, spec hdriver.Spec) (hdriver.Client, error) {
		return New(spec)
	})
}

type client struct {
	base    string
	headers map[string]string
	http    *http.Client

	mu     sync.Mutex
	closed bool
}

func New(spec hdriver.Spec) (hdriver.Client, error) {
	timeout := spec.Timeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &client{
		base:    strings.TrimRight(spec.BaseURL, "/"),
		headers: spec.DefaultHeaders,
		http: &http.Client{
			Timeout:   timeout,
			Transport: middleware.InstrumentTransport(http.DefaultTransport, spec.OTelServiceName),
		},
	}, nil
}

func (c *client) Name() string { return hdriver.DriverStdlib }

func (c *client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	if err := c.checkOpen(); err != nil {
		return nil, err
	}
	r := req.Clone(ctx)
	if c.base != "" && !r.URL.IsAbs() {
		ref := r.URL.String()
		if !strings.HasPrefix(ref, "/") {
			ref = "/" + ref
		}
		u, err := url.Parse(c.base + ref)
		if err != nil {
			return nil, err
		}
		r.URL = u
	}
	for k, v := range c.headers {
		if r.Header.Get(k) == "" {
			r.Header.Set(k, v)
		}
	}
	return c.http.Do(r)
}

func (c *client) Shutdown(context.Context) error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	return nil
}

func (c *client) checkOpen() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return hdriver.ErrClosed
	}
	return nil
}
