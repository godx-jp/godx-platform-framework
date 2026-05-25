// Package azure implements an Azure Blob Storage-backed storage
// driver.
//
// HEAVY driver — depends on github.com/Azure/azure-sdk-for-go/sdk
// /storage/azblob. Consumers opt in with a blank import:
//
//	import _ "github.com/godx-jp/godx-platform-framework/storage/drivers/azure"
//
// Configuration (see docs/CONFIGURATION.md § Storage):
//
//	STORAGE_DISK_<NAME>_DRIVER=azure
//	STORAGE_DISK_<NAME>_ENDPOINT=https://<account>.blob.core.windows.net
//	STORAGE_DISK_<NAME>_BUCKET=my-container
//	# Auth — either shared-key (preferred for SAS issuance):
//	STORAGE_DISK_<NAME>_ACCESS_KEY=<storage account name>
//	STORAGE_DISK_<NAME>_SECRET_KEY=<storage account key>
//	# …or rely on the DefaultAzureCredential chain (AZURE_*, managed
//	# identity, az login). Note: without a shared-key credential the
//	# driver cannot issue SAS URLs locally and TemporaryURL returns
//	# driver.ErrNotSupported.
//	# STORAGE_DISK_<NAME>_PUBLIC_URL=https://<account>.blob.core.windows.net/<container>
//
// Notes
//
//   - Azure exposes per-CONTAINER public access (not per-blob). The
//     driver therefore ignores per-write visibility flags — set
//     container-level access via the Azure portal or `az storage
//     container set-permission`, and supply a PublicURL when you want
//     disk.URL() to work.
//   - SAS URL generation requires a shared-key credential
//     (ACCESS_KEY + SECRET_KEY). User-delegation SAS via OAuth is
//     out of scope for v0.6.2.
package azure

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/blob"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/bloberror"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/container"
	"github.com/Azure/azure-sdk-for-go/sdk/storage/azblob/sas"

	stordriver "github.com/godx-jp/godx-platform-framework/storage/driver"
)

func init() {
	stordriver.Register(stordriver.DriverAzure, construct)
}

// Name is exported so callers can reference the driver name without
// hard-coding a string.
const Name = stordriver.DriverAzure

func construct(_ context.Context, spec stordriver.Spec) (stordriver.Driver, error) {
	if strings.TrimSpace(spec.Bucket) == "" {
		return nil, fmt.Errorf("azure: bucket (container) is required")
	}
	if strings.TrimSpace(spec.Endpoint) == "" {
		return nil, fmt.Errorf("azure: endpoint is required (e.g. https://<account>.blob.core.windows.net)")
	}

	var (
		client    *azblob.Client
		sharedKey *azblob.SharedKeyCredential
		err       error
	)

	if spec.AccessKey != "" && spec.SecretKey != "" {
		sharedKey, err = azblob.NewSharedKeyCredential(spec.AccessKey, spec.SecretKey)
		if err != nil {
			return nil, fmt.Errorf("azure: shared-key credential: %w", err)
		}
		client, err = azblob.NewClientWithSharedKeyCredential(spec.Endpoint, sharedKey, nil)
	} else {
		cred, cerr := azidentity.NewDefaultAzureCredential(nil)
		if cerr != nil {
			return nil, fmt.Errorf("azure: default credential: %w", cerr)
		}
		client, err = azblob.NewClient(spec.Endpoint, cred, nil)
	}
	if err != nil {
		return nil, fmt.Errorf("azure: new client: %w", err)
	}

	return &impl{
		client:    client,
		container: spec.Bucket,
		sharedKey: sharedKey,
		publicURL: strings.TrimRight(spec.PublicURL, "/"),
	}, nil
}

type impl struct {
	client    *azblob.Client
	container string
	publicURL string
	sharedKey *azblob.SharedKeyCredential // nil unless shared-key auth (required for SAS)

	shutdownOnce sync.Once
}

func cleanKey(key string) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("azure: empty key")
	}
	c := path.Clean("/" + strings.ReplaceAll(key, `\`, "/"))
	if c == "/" {
		return "", fmt.Errorf("azure: empty key after clean")
	}
	return strings.TrimPrefix(c, "/"), nil
}

func (d *impl) NewReader(ctx context.Context, key string) (io.ReadCloser, error) {
	k, err := cleanKey(key)
	if err != nil {
		return nil, err
	}
	resp, err := d.client.DownloadStream(ctx, d.container, k, nil)
	if err != nil {
		return nil, translateError(err, key)
	}
	return resp.Body, nil
}

func (d *impl) NewWriter(ctx context.Context, key string, opts stordriver.WriteOptions) (io.WriteCloser, error) {
	k, err := cleanKey(key)
	if err != nil {
		return nil, err
	}
	return newPipeWriter(ctx, d, k, opts), nil
}

// pipeWriter streams writes into UploadStream so callers can use the
// same io.WriteCloser pattern that the local and s3 drivers expose.
type pipeWriter struct {
	pw     *io.PipeWriter
	done   chan error
	closed bool
}

func (w *pipeWriter) Write(p []byte) (int, error) {
	if w.closed {
		return 0, fmt.Errorf("azure: write after close")
	}
	return w.pw.Write(p)
}

func (w *pipeWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	if err := w.pw.Close(); err != nil {
		<-w.done
		return err
	}
	return <-w.done
}

func newPipeWriter(ctx context.Context, d *impl, key string, opts stordriver.WriteOptions) *pipeWriter {
	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() {
		defer pr.Close()
		uploadOpts := &azblob.UploadStreamOptions{}
		var meta map[string]*string
		if len(opts.Metadata) > 0 {
			meta = make(map[string]*string, len(opts.Metadata))
			for k, v := range opts.Metadata {
				vv := v
				meta[k] = &vv
			}
			uploadOpts.Metadata = meta
		}
		if hh := newBlobHTTPHeaders(opts); hh != nil {
			uploadOpts.HTTPHeaders = hh
		}
		_, err := d.client.UploadStream(ctx, d.container, key, pr, uploadOpts)
		done <- err
	}()
	return &pipeWriter{pw: pw, done: done}
}

// newBlobHTTPHeaders builds the SDK header struct from generic
// write options. Returns nil when no header is set so the SDK skips
// the metadata round trip.
func newBlobHTTPHeaders(opts stordriver.WriteOptions) *blob.HTTPHeaders {
	if opts.ContentType == "" && opts.CacheControl == "" {
		return nil
	}
	hh := &blob.HTTPHeaders{}
	if opts.ContentType != "" {
		ct := opts.ContentType
		hh.BlobContentType = &ct
	}
	if opts.CacheControl != "" {
		cc := opts.CacheControl
		hh.BlobCacheControl = &cc
	}
	return hh
}

func (d *impl) Delete(ctx context.Context, key string) error {
	k, err := cleanKey(key)
	if err != nil {
		return err
	}
	if _, err := d.client.DeleteBlob(ctx, d.container, k, nil); err != nil {
		return translateError(err, key)
	}
	return nil
}

func (d *impl) Exists(ctx context.Context, key string) (bool, error) {
	k, err := cleanKey(key)
	if err != nil {
		return false, err
	}
	blobClient := d.client.ServiceClient().NewContainerClient(d.container).NewBlobClient(k)
	_, err = blobClient.GetProperties(ctx, nil)
	if err == nil {
		return true, nil
	}
	if isNotFound(err) {
		return false, nil
	}
	return false, translateError(err, key)
}

func (d *impl) Attributes(ctx context.Context, key string) (stordriver.Attributes, error) {
	k, err := cleanKey(key)
	if err != nil {
		return stordriver.Attributes{}, err
	}
	blobClient := d.client.ServiceClient().NewContainerClient(d.container).NewBlobClient(k)
	resp, err := blobClient.GetProperties(ctx, nil)
	if err != nil {
		return stordriver.Attributes{}, translateError(err, key)
	}
	out := stordriver.Attributes{
		Visibility: stordriver.VisibilityPrivate, // Azure scopes visibility at container, not blob
	}
	if resp.ContentLength != nil {
		out.Size = *resp.ContentLength
	}
	if resp.LastModified != nil {
		out.LastModified = *resp.LastModified
	}
	if resp.ContentType != nil {
		out.ContentType = *resp.ContentType
	}
	if resp.ETag != nil {
		out.ETag = strings.Trim(string(*resp.ETag), `"`)
	}
	if len(resp.Metadata) > 0 {
		out.Metadata = make(map[string]string, len(resp.Metadata))
		for k, v := range resp.Metadata {
			if v != nil {
				out.Metadata[k] = *v
			}
		}
	}
	return out, nil
}

func (d *impl) List(ctx context.Context, prefix string) ([]stordriver.Entry, error) {
	p := strings.TrimPrefix(path.Clean("/"+prefix), "/")
	if prefix == "" || prefix == "/" {
		p = ""
	}
	if p != "" && !strings.HasSuffix(p, "/") {
		p += "/"
	}
	pager := d.client.ServiceClient().NewContainerClient(d.container).NewListBlobsHierarchyPager("/", &container.ListBlobsHierarchyOptions{
		Prefix: stringPtr(p),
	})
	var entries []stordriver.Entry
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("azure: list %q: %w", prefix, err)
		}
		for _, blob := range page.Segment.BlobItems {
			if blob == nil || blob.Name == nil {
				continue
			}
			e := stordriver.Entry{Key: *blob.Name}
			if blob.Properties != nil {
				if blob.Properties.ContentLength != nil {
					e.Size = *blob.Properties.ContentLength
				}
				if blob.Properties.LastModified != nil {
					e.LastModified = *blob.Properties.LastModified
				}
			}
			entries = append(entries, e)
		}
		for _, p := range page.Segment.BlobPrefixes {
			if p == nil || p.Name == nil {
				continue
			}
			key := strings.TrimSuffix(*p.Name, "/")
			entries = append(entries, stordriver.Entry{Key: key, IsDir: true})
		}
	}
	return entries, nil
}

func stringPtr(s string) *string { return &s }

func (d *impl) URL(key string) (string, error) {
	if d.publicURL == "" {
		return "", fmt.Errorf("%w: configure Spec.PublicURL to enable URL()", stordriver.ErrNotSupported)
	}
	k, err := cleanKey(key)
	if err != nil {
		return "", err
	}
	parts := strings.Split(k, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return d.publicURL + "/" + strings.Join(parts, "/"), nil
}

func (d *impl) SignedURL(_ context.Context, key string, expires time.Duration) (string, error) {
	if d.sharedKey == nil {
		return "", fmt.Errorf("%w: azure SAS requires shared-key credentials (set STORAGE_DISK_<NAME>_ACCESS_KEY + _SECRET_KEY)", stordriver.ErrNotSupported)
	}
	if expires <= 0 {
		expires = 15 * time.Minute
	}
	k, err := cleanKey(key)
	if err != nil {
		return "", err
	}
	sasValues := sas.BlobSignatureValues{
		Protocol:      sas.ProtocolHTTPS,
		ExpiryTime:    time.Now().UTC().Add(expires),
		ContainerName: d.container,
		BlobName:      k,
		Permissions:   (&sas.BlobPermissions{Read: true}).String(),
	}
	queryParams, err := sasValues.SignWithSharedKey(d.sharedKey)
	if err != nil {
		return "", fmt.Errorf("azure: sign SAS %q: %w", key, err)
	}
	// Compose <endpoint>/<container>/<blob>?<sas>
	endpoint := strings.TrimRight(d.client.URL(), "/")
	parts := strings.Split(k, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return fmt.Sprintf("%s/%s/%s?%s", endpoint, url.PathEscape(d.container), strings.Join(parts, "/"), queryParams.Encode()), nil
}

func (d *impl) Shutdown(_ context.Context) error {
	d.shutdownOnce.Do(func() {})
	return nil
}

// ── error translation ────────────────────────────────────────────────

func translateError(err error, key string) error {
	if err == nil {
		return nil
	}
	if isNotFound(err) {
		return fmt.Errorf("%w: %s", stordriver.ErrNotFound, key)
	}
	return fmt.Errorf("azure: %w", err)
}

func isNotFound(err error) bool {
	return bloberror.HasCode(err, bloberror.BlobNotFound) ||
		bloberror.HasCode(err, bloberror.ContainerNotFound) ||
		bloberror.HasCode(err, bloberror.ResourceNotFound)
}

// silence unused (kept available for future SDK helpers)
var (
	_ = bytes.NewReader
	_ = errors.Is
)
