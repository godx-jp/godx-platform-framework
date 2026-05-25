// Package mock is an in-memory HTTP client for tests — records
// requests and returns configured responses.
package mock

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sync"

	hdriver "github.com/godx-jp/godx-platform-framework/httpclient/driver"
)

func init() {
	hdriver.Register(hdriver.DriverMock, func(ctx context.Context, spec hdriver.Spec) (hdriver.Client, error) {
		return New(), nil
	})
}

type response struct {
	status int
	body   []byte
	header http.Header
}

type client struct {
	mu        sync.Mutex
	closed    bool
	responses []response
	requests  []*http.Request
	idx       int
}

// New returns a mock Client with no preset responses (404 by default).
func New() *client { return &client{} }

func (c *client) Name() string { return hdriver.DriverMock }

// PushResponse queues a response returned on the next Do call.
func (c *client) PushResponse(status int, body []byte, header http.Header) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.responses = append(c.responses, response{status: status, body: body, header: header})
}

// Requests returns a copy of recorded requests.
func (c *client) Requests() []*http.Request {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*http.Request, len(c.requests))
	copy(out, c.requests)
	return out
}

func (c *client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil, hdriver.ErrClosed
	}
	r := req.Clone(ctx)
	c.requests = append(c.requests, r)
	var resp response
	if c.idx < len(c.responses) {
		resp = c.responses[c.idx]
		c.idx++
	} else {
		resp = response{status: http.StatusNotFound, body: []byte("not found")}
	}
	h := http.Header{}
	for k, vv := range resp.header {
		h[k] = append([]string(nil), vv...)
	}
	return &http.Response{
		StatusCode: resp.status,
		Status:     http.StatusText(resp.status),
		Header:     h,
		Body:       io.NopCloser(bytes.NewReader(resp.body)),
		Request:    r,
	}, nil
}

func (c *client) Shutdown(context.Context) error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	return nil
}
