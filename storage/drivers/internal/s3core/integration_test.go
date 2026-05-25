//go:build integration

// Integration smoke test for the S3 protocol driver. Hits a REAL S3
// endpoint (MinIO by default) — does NOT run in normal `go test`.
//
// Boot a MinIO in another shell:
//
//	docker run --rm -d --name mc -p 9000:9000 -p 9001:9001 \
//	    -e MINIO_ROOT_USER=minioadmin -e MINIO_ROOT_PASSWORD=minioadmin \
//	    quay.io/minio/minio server /data --console-address :9001
//	docker exec mc mc alias set local http://127.0.0.1:9000 minioadmin minioadmin
//	docker exec mc mc mb local/godx-test
//
// Then run:
//
//	go test -tags integration -run TestS3Core_Integration_MinIO ./storage/drivers/internal/s3core/
//
// Env overrides (all optional):
//
//	S3CORE_TEST_ENDPOINT     default http://127.0.0.1:9000
//	S3CORE_TEST_BUCKET       default godx-test
//	S3CORE_TEST_ACCESS_KEY   default minioadmin
//	S3CORE_TEST_SECRET_KEY   default minioadmin
//	S3CORE_TEST_REGION       default us-east-1
package s3core_test

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"testing"
	"time"

	stordriver "github.com/godx-jp/godx-platform-framework/storage/driver"
	"github.com/godx-jp/godx-platform-framework/storage/drivers/internal/s3core"
)

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func mustReachable(t *testing.T, endpoint string) {
	t.Helper()
	host := endpoint
	for _, prefix := range []string{"http://", "https://"} {
		if len(host) > len(prefix) && host[:len(prefix)] == prefix {
			host = host[len(prefix):]
			break
		}
	}
	conn, err := net.DialTimeout("tcp", host, 2*time.Second)
	if err != nil {
		t.Skipf("integration: %s unreachable (%v); skipping. boot MinIO per file header.", endpoint, err)
	}
	_ = conn.Close()
}

func TestS3Core_Integration_MinIO(t *testing.T) {
	endpoint := envOr("S3CORE_TEST_ENDPOINT", "http://127.0.0.1:9000")
	mustReachable(t, endpoint)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	spec := stordriver.Spec{
		Bucket:       envOr("S3CORE_TEST_BUCKET", "godx-test"),
		Region:       envOr("S3CORE_TEST_REGION", "us-east-1"),
		Endpoint:     endpoint,
		AccessKey:    envOr("S3CORE_TEST_ACCESS_KEY", "minioadmin"),
		SecretKey:    envOr("S3CORE_TEST_SECRET_KEY", "minioadmin"),
		UsePathStyle: true,
		PublicURL:    endpoint + "/" + envOr("S3CORE_TEST_BUCKET", "godx-test"),
	}
	ctor := s3core.NewConstructor(s3core.ProfileMinIO)
	d, err := ctor(ctx, spec)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	t.Cleanup(func() { _ = d.Shutdown(context.Background()) })

	key := "integration/hello.txt"
	w, err := d.NewWriter(ctx, key, stordriver.WriteOptions{ContentType: "text/plain"})
	if err != nil {
		t.Fatalf("writer: %v", err)
	}
	if _, err := w.Write([]byte("hello-from-godx")); err != nil {
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
	if string(body) != "hello-from-godx" {
		t.Fatalf("body = %q", body)
	}

	attr, err := d.Attributes(ctx, key)
	if err != nil {
		t.Fatalf("attrs: %v", err)
	}
	if attr.Size != int64(len("hello-from-godx")) {
		t.Fatalf("size = %d", attr.Size)
	}

	signed, err := d.SignedURL(ctx, key, 5*time.Minute)
	if err != nil {
		t.Fatalf("signed: %v", err)
	}
	if signed == "" {
		t.Fatal("empty signed URL")
	}

	if err := d.Delete(ctx, key); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := d.Delete(ctx, key); !errors.Is(err, stordriver.ErrNotFound) {
		t.Fatalf("re-delete: want ErrNotFound, got %v", err)
	}
}
