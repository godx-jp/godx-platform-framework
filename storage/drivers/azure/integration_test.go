//go:build integration

// Integration smoke test for the Azure driver. Hits a real Azure Blob
// endpoint (Azurite by default) — does NOT run in normal `go test`.
//
// Boot Azurite in another shell:
//
//	docker run --rm -d --name azurite -p 10000:10000 \
//	    mcr.microsoft.com/azure-storage/azurite \
//	    azurite-blob --blobHost 0.0.0.0
//
// Azurite ships a well-known dev account (`devstoreaccount1`) with key
// `Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw==`.
// Create a container before running the test:
//
//	docker exec azurite bash -c 'apt-get update -qq && apt-get install -y -qq curl >/dev/null 2>&1; \
//	    curl -X PUT "http://127.0.0.1:10000/devstoreaccount1/godx-test?restype=container"'
//
// Or use the azure-cli / az storage container create.
//
// Then run:
//
//	go test -tags integration -run TestAzure_Integration_Azurite ./storage/drivers/azure/
//
// Env overrides (all optional):
//
//	AZURE_TEST_ENDPOINT     default http://127.0.0.1:10000/devstoreaccount1
//	AZURE_TEST_CONTAINER    default godx-test
//	AZURE_TEST_ACCOUNT      default devstoreaccount1
//	AZURE_TEST_KEY          default Azurite well-known key
package azure_test

import (
	"context"
	"errors"
	"io"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"

	stordriver "github.com/godx-jp/godx-platform-framework/storage/driver"
	_ "github.com/godx-jp/godx-platform-framework/storage/drivers/azure"
)

const azuriteKey = "Eby8vdM02xNOcqFlqUwJPLlmEtlCDXJ1OUzFT50uSRZ6IFsuFq2UVErCz4I6tq/K1SZFPTOtr/KBHBeksoGMGw=="

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
		host = strings.TrimPrefix(endpoint, "http://")
	}
	conn, err := net.DialTimeout("tcp", host, 2*time.Second)
	if err != nil {
		t.Skipf("integration: %s unreachable (%v); skipping. boot Azurite per file header.", endpoint, err)
	}
	_ = conn.Close()
}

func TestAzure_Integration_Azurite(t *testing.T) {
	endpoint := envOr("AZURE_TEST_ENDPOINT", "http://127.0.0.1:10000/devstoreaccount1")
	mustReachable(t, endpoint)

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	containerName := envOr("AZURE_TEST_CONTAINER", "godx-test")
	account := envOr("AZURE_TEST_ACCOUNT", "devstoreaccount1")
	key := envOr("AZURE_TEST_KEY", azuriteKey)

	// Auto-create the container so the test is self-contained.
	cred, err := azblob.NewSharedKeyCredential(account, key)
	if err != nil {
		t.Fatalf("shared key: %v", err)
	}
	bootClient, err := azblob.NewClientWithSharedKeyCredential(endpoint, cred, nil)
	if err != nil {
		t.Fatalf("boot client: %v", err)
	}
	if _, err := bootClient.CreateContainer(ctx, containerName, nil); err != nil {
		if !bloberror.HasCode(err, bloberror.ContainerAlreadyExists) {
			t.Fatalf("create container: %v", err)
		}
	}

	spec := stordriver.Spec{
		Name:      stordriver.DriverAzure,
		Endpoint:  endpoint,
		Bucket:    containerName,
		AccessKey: account,
		SecretKey: key,
	}
	d, err := stordriver.New(ctx, spec)
	if err != nil {
		t.Fatalf("construct: %v", err)
	}
	t.Cleanup(func() { _ = d.Shutdown(context.Background()) })

	blobKey := "integration/hello.txt"
	w, err := d.NewWriter(ctx, blobKey, stordriver.WriteOptions{ContentType: "text/plain"})
	if err != nil {
		t.Fatalf("writer: %v", err)
	}
	if _, err := w.Write([]byte("hello-from-godx-azure")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	ok, err := d.Exists(ctx, blobKey)
	if err != nil || !ok {
		t.Fatalf("exists: ok=%v err=%v", ok, err)
	}

	r, err := d.NewReader(ctx, blobKey)
	if err != nil {
		t.Fatalf("reader: %v", err)
	}
	body, _ := io.ReadAll(r)
	_ = r.Close()
	if string(body) != "hello-from-godx-azure" {
		t.Fatalf("body = %q", body)
	}

	attr, err := d.Attributes(ctx, blobKey)
	if err != nil {
		t.Fatalf("attrs: %v", err)
	}
	if attr.Size != int64(len("hello-from-godx-azure")) {
		t.Fatalf("size = %d", attr.Size)
	}

	signed, err := d.SignedURL(ctx, blobKey, 5*time.Minute)
	if err != nil {
		t.Fatalf("signed: %v", err)
	}
	if signed == "" {
		t.Fatal("empty signed URL")
	}

	if err := d.Delete(ctx, blobKey); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if err := d.Delete(ctx, blobKey); !errors.Is(err, stordriver.ErrNotFound) {
		t.Fatalf("re-delete: want ErrNotFound, got %v", err)
	}
}
