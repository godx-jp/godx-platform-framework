package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	hdriver "github.com/godx-jp/godx-platform-framework/httpclient/driver"
)

// Client is the public handle wrapping a driver.Client with
// convenience methods.
type Client struct {
	inner hdriver.Client
	base  string
}

// Wrap returns a Client around c.
func Wrap(c hdriver.Client) *Client {
	return &Client{inner: c}
}

// WrapWithBase sets a relative-path base for Get/Post helpers.
func WrapWithBase(c hdriver.Client, base string) *Client {
	return &Client{inner: c, base: strings.TrimRight(base, "/")}
}

func (c *Client) Name() string { return c.inner.Name() }

// Do forwards to the underlying driver.
func (c *Client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	return c.inner.Do(ctx, req)
}

// Get issues a GET request.
func (c *Client) Get(ctx context.Context, path string) (*http.Response, error) {
	return c.doMethod(ctx, http.MethodGet, path, nil, nil)
}

// Post issues a POST with an optional body reader and content type.
func (c *Client) Post(ctx context.Context, path string, body io.Reader, contentType string) (*http.Response, error) {
	h := http.Header{}
	if contentType != "" {
		h.Set("Content-Type", contentType)
	}
	return c.doMethod(ctx, http.MethodPost, path, body, h)
}

// PostJSON marshals v as JSON and POSTs it.
func (c *Client) PostJSON(ctx context.Context, path string, v any) (*http.Response, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	return c.Post(ctx, path, bytes.NewReader(b), "application/json")
}

func (c *Client) doMethod(ctx context.Context, method, path string, body io.Reader, hdr http.Header) (*http.Response, error) {
	u, err := c.resolve(path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, u, body)
	if err != nil {
		return nil, err
	}
	for k, vv := range hdr {
		for _, v := range vv {
			req.Header.Add(k, v)
		}
	}
	return c.inner.Do(ctx, req)
}

func (c *Client) resolve(path string) (string, error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path, nil
	}
	if c.base == "" {
		return path, nil
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	_, err := url.Parse(c.base + path)
	if err != nil {
		return "", fmt.Errorf("httpclient: resolve: %w", err)
	}
	return c.base + path, nil
}

// Shutdown releases driver resources.
func (c *Client) Shutdown(ctx context.Context) error { return c.inner.Shutdown(ctx) }

// JSON is a helper content-type constant.
const JSON = "application/json"
