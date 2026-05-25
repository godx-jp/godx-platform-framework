//go:build integration

// Integration smoke test for the GCS driver. Hits a real GCS-compatible
// endpoint (fake-gcs-server by default) — does NOT run in normal
// `go test`.
//
// Boot fake-gcs-server in another shell:
//
//	docker run --rm -d --name fakegcs -p 4443:4443 \
//	    fsouza/fake-gcs-server -scheme http -public-host 127.0.0.1:4443
//	# Create a bucket the test will use:
//	curl -X POST -H 'Content-Type: application/json' \
//	    --data '{"name":"godx-test"}' \
//	    http://127.0.0.1:4443/storage/v1/b
//
// Then run:
//
//	STORAGE_API_ENDPOINT_OVERRIDE=http://127.0.0.1:4443/storage/v1/ \
//	go test -tags integration -run TestGCS_Integration_FakeServer ./storage/drivers/gcs/
//
// Env overrides (all optional):
//
//	GCS_TEST_ENDPOINT   default http://127.0.0.1:4443/storage/v1/
//	GCS_TEST_BUCKET     default godx-test
package gcs_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"testing"
	"time"

	stordriver "github.com/godx-jp/godx-platform-framework/storage/driver"
	_ "github.com/godx-jp/godx-platform-framework/storage/drivers/gcs"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustReachable(t *testing.T, endpoint string) {
	t.Helper()
	u, err := url.Parse(endpoint)
	if err != nil {
		t.Skipf("integration: bad endpoint %q: %v", endpoint, err)
	}
	host := u.Host
	if host == "" {
		host = endpoint
	}
	conn, err := net.DialTimeout("tcp", host, 2*time.Second)
	if err != nil {
		t.Skipf("integration: %s unreachable (%v); skipping. boot fake-gcs-server per file header.", endpoint, err)
	}
	_ = conn.Close()
}

func TestGCS_Integration_FakeServer(t *testing.T) {
	endpoint := envOr("GCS_TEST_ENDPOINT", "http://127.0.0.1:4443/storage/v1/")
	mustReachable(t, endpoint)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	spec := stordriver.Spec{
		Name:     stordriver.DriverGCS,
		Bucket:   envOr("GCS_TEST_BUCKET", "godx-test"),
		Endpoint: endpoint,
	}
	d, err := stordriver.New(ctx, spec)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	t.Cleanup(func() { _ = d.Shutdown(context.Background()) })

	key := "integration/hello.txt"

	w, err := d.NewWriter(ctx, key, stordriver.WriteOptions{ContentType: "text/plain"})
	if err != nil {
		t.Fatalf("writer: %v", err)
	}
	if _, err := w.Write([]byte("hello-from-godx-gcs")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	ok, err := d.Exists(ctx, key)
	if err != nil || !ok {
		t.Fatalf("exists: ok=%v err=%v", ok, err)
	}

	r, err := d.NewReader(ctx, key)
	if err != nil {
		t.Fatalf("reader: %v", err)
	}
	body, _ := io.ReadAll(r)
	_ = r.Close()
	if string(body) != "hello-from-godx-gcs" {
		t.Fatalf("body = %q", body)
	}

	if err := d.Delete(ctx, key); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := d.Delete(ctx, key); !errors.Is(err, stordriver.ErrNotFound) {
		t.Fatalf("re-delete: want ErrNotFound, got %v", err)
	}
}
