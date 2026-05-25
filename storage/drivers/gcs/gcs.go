// Package gcs implements a Google Cloud Storage-backed storage driver.
//
// HEAVY driver — depends on cloud.google.com/go/storage. Consumers
// opt in with a blank import:
//
//	import _ "github.com/godx-jp/godx-platform-framework/storage/drivers/gcs"
//
// Configuration (see docs/CONFIGURATION.md § Storage):
//
//	STORAGE_DISK_<NAME>_DRIVER=gcs
//	STORAGE_DISK_<NAME>_BUCKET=my-bucket
//	# Credentials are resolved by Application Default Credentials:
//	# - GOOGLE_APPLICATION_CREDENTIALS=/path/to/sa.json
//	# - gcloud auth application-default login
//	# - Workload Identity / GKE / GCE metadata server
//	# STORAGE_DISK_<NAME>_PUBLIC_URL=https://storage.googleapis.com/my-bucket
//	# STORAGE_DISK_<NAME>_ENDPOINT=http://localhost:4443/storage/v1/  # for fake-gcs-server
//
// Notes
//
//   - GCS buckets with Uniform Bucket-Level Access (UBLA, the default
//     for new buckets) reject per-object ACLs. The driver only attempts
//     to set PredefinedACL when the caller requests an explicit
//     visibility — leaving the default unset lets UBLA buckets work
//     transparently. Use bucket-level IAM for public exposure on UBLA.
//   - SignedURL requires service-account credentials with a private key
//     (typically GOOGLE_APPLICATION_CREDENTIALS pointing at a JSON key).
//     Metadata-server credentials cannot sign locally; in that case
//     enable IAM SignBlob — out of scope for v0.6.2.
package gcs

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
	"time"

	gcs "cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"

	stordriver "github.com/godx-jp/godx-platform-framework/storage/driver"
)

func init() {
	stordriver.Register(stordriver.DriverGCS, construct)
}

// Name is exported so callers can reference the driver name without
// hard-coding a string.
const Name = stordriver.DriverGCS

func construct(ctx context.Context, spec stordriver.Spec) (stordriver.Driver, error) {
	if strings.TrimSpace(spec.Bucket) == "" {
		return nil, fmt.Errorf("gcs: bucket is required")
	}

	var opts []option.ClientOption
	if spec.Endpoint != "" {
		// Custom endpoint pattern is used for emulators
		// (fake-gcs-server, etc.). Skip auth in that case.
		opts = append(opts, option.WithEndpoint(spec.Endpoint), option.WithoutAuthentication())
	}

	client, err := gcs.NewClient(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("gcs: new client: %w", err)
	}

	return &impl{
		client:     client,
		bucket:     spec.Bucket,
		publicURL:  strings.TrimRight(spec.PublicURL, "/"),
		defaultVis: defaultVisibility(spec.DefaultVisibility),
	}, nil
}

func defaultVisibility(v stordriver.Visibility) stordriver.Visibility {
	if v.IsValid() {
		return v
	}
	return stordriver.VisibilityPrivate
}

type impl struct {
	client     *gcs.Client
	bucket     string
	publicURL  string
	defaultVis stordriver.Visibility
}

func cleanKey(key string) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("gcs: empty key")
	}
	c := path.Clean("/" + strings.ReplaceAll(key, `\`, "/"))
	if c == "/" {
		return "", fmt.Errorf("gcs: empty key after clean")
	}
	return strings.TrimPrefix(c, "/"), nil
}

func (d *impl) obj(key string) (*gcs.ObjectHandle, string, error) {
	k, err := cleanKey(key)
	if err != nil {
		return nil, "", err
	}
	return d.client.Bucket(d.bucket).Object(k), k, nil
}

func (d *impl) NewReader(ctx context.Context, key string) (io.ReadCloser, error) {
	o, _, err := d.obj(key)
	if err != nil {
		return nil, err
	}
	r, err := o.NewReader(ctx)
	if err != nil {
		return nil, translateError(err, key)
	}
	return r, nil
}

func (d *impl) NewWriter(ctx context.Context, key string, opts stordriver.WriteOptions) (io.WriteCloser, error) {
	o, _, err := d.obj(key)
	if err != nil {
		return nil, err
	}
	w := o.NewWriter(ctx)
	if opts.ContentType != "" {
		w.ContentType = opts.ContentType
	}
	if opts.CacheControl != "" {
		w.CacheControl = opts.CacheControl
	}
	if len(opts.Metadata) > 0 {
		w.Metadata = cloneMeta(opts.Metadata)
	}
	// Only set the predefined ACL when the caller explicitly requested
	// a visibility. UBLA buckets reject per-object ACLs, so leaving the
	// field unset keeps them working without configuration.
	if opts.Visibility.IsValid() {
		w.PredefinedACL = visibilityToACL(opts.Visibility)
	}
	return w, nil
}

func cloneMeta(m map[string]string) map[string]string {
	out := make(map[string]string, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}

func visibilityToACL(v stordriver.Visibility) string {
	if v == stordriver.VisibilityPublic {
		return "publicRead"
	}
	return "private"
}

func (d *impl) Delete(ctx context.Context, key string) error {
	o, _, err := d.obj(key)
	if err != nil {
		return err
	}
	if err := o.Delete(ctx); err != nil {
		return translateError(err, key)
	}
	return nil
}

func (d *impl) Exists(ctx context.Context, key string) (bool, error) {
	o, _, err := d.obj(key)
	if err != nil {
		return false, err
	}
	_, err = o.Attrs(ctx)
	if err == nil {
		return true, nil
	}
	if errors.Is(err, gcs.ErrObjectNotExist) {
		return false, nil
	}
	return false, translateError(err, key)
}

func (d *impl) Attributes(ctx context.Context, key string) (stordriver.Attributes, error) {
	o, _, err := d.obj(key)
	if err != nil {
		return stordriver.Attributes{}, err
	}
	attrs, err := o.Attrs(ctx)
	if err != nil {
		return stordriver.Attributes{}, translateError(err, key)
	}
	out := stordriver.Attributes{
		Size:         attrs.Size,
		LastModified: attrs.Updated,
		ContentType:  attrs.ContentType,
		ETag:         attrs.Etag,
		Visibility:   stordriver.VisibilityPrivate, // GCS doesn't expose ACL reliably under UBLA
	}
	if len(attrs.Metadata) > 0 {
		out.Metadata = cloneMeta(attrs.Metadata)
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
	it := d.client.Bucket(d.bucket).Objects(ctx, &gcs.Query{
		Prefix:    p,
		Delimiter: "/",
	})
	var entries []stordriver.Entry
	for {
		attrs, err := it.Next()
		if errors.Is(err, iterator.Done) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("gcs: list %q: %w", prefix, err)
		}
		if attrs.Prefix != "" {
			key := strings.TrimSuffix(attrs.Prefix, "/")
			entries = append(entries, stordriver.Entry{Key: key, IsDir: true})
			continue
		}
		entries = append(entries, stordriver.Entry{
			Key:          attrs.Name,
			Size:         attrs.Size,
			LastModified: attrs.Updated,
		})
	}
	return entries, nil
}

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

// maxSignedURLTTL caps how long a signed URL may stay valid. 7 days
// matches S3 SigV4's own hard limit; a leaked URL therefore cannot grant
// access beyond a week even if a caller (or misconfig) requests longer.
const maxSignedURLTTL = 7 * 24 * time.Hour

// clampTTL floors a non-positive expiry to a 15-minute default and caps an
// over-long one to maxSignedURLTTL. It never errors — it clamps and proceeds.
func clampTTL(d time.Duration) time.Duration {
	if d <= 0 {
		return 15 * time.Minute
	}
	if d > maxSignedURLTTL {
		return maxSignedURLTTL
	}
	return d
}

func (d *impl) SignedURL(_ context.Context, key string, expires time.Duration) (string, error) {
	expires = clampTTL(expires)
	k, err := cleanKey(key)
	if err != nil {
		return "", err
	}
	signed, err := d.client.Bucket(d.bucket).SignedURL(k, &gcs.SignedURLOptions{
		Method:  "GET",
		Expires: time.Now().Add(expires),
		Scheme:  gcs.SigningSchemeV4,
	})
	if err != nil {
		return "", fmt.Errorf("%w: gcs.SignedURL %q (requires a service-account JSON key — metadata-server creds cannot sign locally): %v",
			stordriver.ErrNotSupported, key, err)
	}
	return signed, nil
}

func (d *impl) Shutdown(_ context.Context) error {
	return d.client.Close()
}

func translateError(err error, key string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, gcs.ErrObjectNotExist) {
		return fmt.Errorf("%w: %s", stordriver.ErrNotFound, key)
	}
	return fmt.Errorf("gcs: %w", err)
}
