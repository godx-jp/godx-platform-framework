// Package memory implements an in-memory storage.Driver useful for
// tests and ephemeral fixtures. Objects live for the lifetime of the
// driver and are discarded on Shutdown.
//
// The driver is "light" — it depends only on the standard library and
// is auto-registered when the parent storage package is imported.
package memory

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/godx-jp/godx-platform-framework/storage/driver"
)

func init() {
	driver.Register(driver.DriverMemory, construct)
}

// Name is exported so callers can reference the driver name without
// hard-coding a string.
const Name = driver.DriverMemory

func construct(_ context.Context, s driver.Spec) (driver.Driver, error) {
	vis := s.DefaultVisibility
	if !vis.IsValid() {
		vis = driver.VisibilityPrivate
	}
	return &impl{
		objects:    map[string]*object{},
		defaultVis: vis,
	}, nil
}

type object struct {
	data         []byte
	contentType  string
	visibility   driver.Visibility
	metadata     map[string]string
	lastModified time.Time
}

type impl struct {
	mu         sync.RWMutex
	objects    map[string]*object
	defaultVis driver.Visibility
}

func cleanKey(key string) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("memory: empty key")
	}
	c := path.Clean("/" + strings.ReplaceAll(key, `\`, "/"))
	if c == "/" {
		return "", fmt.Errorf("memory: empty key after clean")
	}
	return strings.TrimPrefix(c, "/"), nil
}

func (d *impl) NewReader(_ context.Context, key string) (io.ReadCloser, error) {
	k, err := cleanKey(key)
	if err != nil {
		return nil, err
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	o, ok := d.objects[k]
	if !ok {
		return nil, fmt.Errorf("%w: %s", driver.ErrNotFound, key)
	}
	return io.NopCloser(bytes.NewReader(o.data)), nil
}

func (d *impl) NewWriter(_ context.Context, key string, opts driver.WriteOptions) (io.WriteCloser, error) {
	k, err := cleanKey(key)
	if err != nil {
		return nil, err
	}
	vis := opts.Visibility
	if !vis.IsValid() {
		vis = d.defaultVis
	}
	return &writer{
		driver:      d,
		key:         k,
		buf:         &bytes.Buffer{},
		contentType: opts.ContentType,
		visibility:  vis,
		metadata:    cloneMeta(opts.Metadata),
	}, nil
}

type writer struct {
	driver      *impl
	key         string
	buf         *bytes.Buffer
	contentType string
	visibility  driver.Visibility
	metadata    map[string]string
	closed      bool
}

func (w *writer) Write(p []byte) (int, error) {
	if w.closed {
		return 0, fmt.Errorf("memory: write after close")
	}
	return w.buf.Write(p)
}

func (w *writer) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	w.driver.mu.Lock()
	defer w.driver.mu.Unlock()
	w.driver.objects[w.key] = &object{
		data:         append([]byte(nil), w.buf.Bytes()...),
		contentType:  w.contentType,
		visibility:   w.visibility,
		metadata:     w.metadata,
		lastModified: time.Now().UTC(),
	}
	return nil
}

func cloneMeta(m map[string]string) map[string]string {
	if len(m) == 0 {
		return nil
	}
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func (d *impl) Delete(_ context.Context, key string) error {
	k, err := cleanKey(key)
	if err != nil {
		return err
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, ok := d.objects[k]; !ok {
		return fmt.Errorf("%w: %s", driver.ErrNotFound, key)
	}
	delete(d.objects, k)
	return nil
}

func (d *impl) Exists(_ context.Context, key string) (bool, error) {
	k, err := cleanKey(key)
	if err != nil {
		return false, err
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	_, ok := d.objects[k]
	return ok, nil
}

func (d *impl) Attributes(_ context.Context, key string) (driver.Attributes, error) {
	k, err := cleanKey(key)
	if err != nil {
		return driver.Attributes{}, err
	}
	d.mu.RLock()
	defer d.mu.RUnlock()
	o, ok := d.objects[k]
	if !ok {
		return driver.Attributes{}, fmt.Errorf("%w: %s", driver.ErrNotFound, key)
	}
	return driver.Attributes{
		Size:         int64(len(o.data)),
		LastModified: o.lastModified,
		ContentType:  o.contentType,
		Visibility:   o.visibility,
		Metadata:     cloneMeta(o.metadata),
	}, nil
}

func (d *impl) List(_ context.Context, prefix string) ([]driver.Entry, error) {
	cleaned := strings.TrimPrefix(path.Clean("/"+prefix), "/")
	if prefix == "" || prefix == "/" {
		cleaned = ""
	}
	dirPrefix := cleaned
	if dirPrefix != "" && !strings.HasSuffix(dirPrefix, "/") {
		dirPrefix += "/"
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	seen := map[string]driver.Entry{}
	for k, o := range d.objects {
		if dirPrefix != "" && !strings.HasPrefix(k, dirPrefix) {
			continue
		}
		rest := strings.TrimPrefix(k, dirPrefix)
		if rest == "" {
			continue
		}
		idx := strings.IndexByte(rest, '/')
		if idx == -1 {
			seen[k] = driver.Entry{
				Key:          k,
				Size:         int64(len(o.data)),
				LastModified: o.lastModified,
			}
		} else {
			dirKey := dirPrefix + rest[:idx]
			if _, ok := seen[dirKey]; !ok {
				seen[dirKey] = driver.Entry{Key: dirKey, IsDir: true}
			}
		}
	}
	out := make([]driver.Entry, 0, len(seen))
	for _, e := range seen {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Key < out[j].Key })
	return out, nil
}

func (d *impl) URL(_ string) (string, error) {
	return "", fmt.Errorf("%w: memory driver has no URL surface", driver.ErrNotSupported)
}

func (d *impl) SignedURL(_ context.Context, _ string, _ time.Duration) (string, error) {
	return "", fmt.Errorf("%w: memory driver cannot sign URLs", driver.ErrNotSupported)
}

func (d *impl) SignedPutURL(_ context.Context, _ string, _ time.Duration) (string, error) {
	return "", fmt.Errorf("%w: memory driver cannot sign upload URLs", driver.ErrNotSupported)
}

func (d *impl) Shutdown(_ context.Context) error {
	d.mu.Lock()
	d.objects = map[string]*object{}
	d.mu.Unlock()
	return nil
}
