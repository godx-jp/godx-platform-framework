// Package s3core implements the shared S3-protocol storage driver used
// by both the AWS S3 and MinIO wrappers. AWS S3, MinIO, Cloudflare R2,
// DigitalOcean Spaces and every other S3-compatible store speak the
// same wire protocol, so a single implementation parameterised by
// "compatibility profile" covers all of them.
//
// This package lives under storage/drivers/internal/ so it is only
// importable by sibling driver packages. Application code should
// always go through the storage.Driver interface — never import
// s3core directly.
//
// Compatibility profiles (see Profile):
//
//   - ProfileAWS:   virtual-hosted-style addressing, region required,
//                   endpoint optional (defaults to AWS regional URL).
//   - ProfileMinIO: path-style addressing forced, endpoint required,
//                   region defaults to "us-east-1" when unset
//                   (MinIO uses it for signing only).
//
// The Profile only affects defaults; every Spec field is still
// overridable so consumers running MinIO behind a custom domain (or
// running AWS-compatible MinIO Gateway against virtual-hosted DNS)
// remain unblocked.
package s3core

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

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"

	stordriver "github.com/godx-jp/godx-platform-framework/storage/driver"
)

// Profile selects compatibility defaults for a wrapper package.
type Profile string

const (
	ProfileAWS   Profile = "aws"
	ProfileMinIO Profile = "minio"
)

// API is the subset of *s3.Client methods the driver consumes. A small
// interface keeps unit tests simple — they implement only the methods
// they need against an in-memory fake. The real *s3.Client satisfies
// API natively so consumers pass nothing extra.
type API interface {
	GetObject(ctx context.Context, in *s3.GetObjectInput, optFns ...func(*s3.Options)) (*s3.GetObjectOutput, error)
	PutObject(ctx context.Context, in *s3.PutObjectInput, optFns ...func(*s3.Options)) (*s3.PutObjectOutput, error)
	DeleteObject(ctx context.Context, in *s3.DeleteObjectInput, optFns ...func(*s3.Options)) (*s3.DeleteObjectOutput, error)
	HeadObject(ctx context.Context, in *s3.HeadObjectInput, optFns ...func(*s3.Options)) (*s3.HeadObjectOutput, error)
	ListObjectsV2(ctx context.Context, in *s3.ListObjectsV2Input, optFns ...func(*s3.Options)) (*s3.ListObjectsV2Output, error)
}

// Presigner is the subset of s3.PresignClient surfaced for SignedURL.
// Heavy clients can implement an empty stub; tests can supply a fake.
type Presigner interface {
	PresignGetObject(ctx context.Context, in *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*PresignedRequest, error)
}

// PresignedRequest mirrors v4.PresignedHTTPRequest at a narrower
// surface so callers don't pull the v4 package transitively.
type PresignedRequest struct {
	URL string
}

// NewConstructor returns a driver.Constructor that builds an s3core
// driver pre-configured for the supplied profile. Wrapper packages
// (drivers/s3, drivers/minio) call this from their init() with the
// matching profile and register the result against driver names "s3"
// and "minio".
func NewConstructor(profile Profile) stordriver.Constructor {
	return func(ctx context.Context, spec stordriver.Spec) (stordriver.Driver, error) {
		return newDriver(ctx, profile, spec, nil)
	}
}

// NewDriverWithAPI is exported for tests so they can inject an in-memory
// API implementation without touching the registry. Production code
// uses NewConstructor.
func NewDriverWithAPI(profile Profile, spec stordriver.Spec, api API, presigner Presigner) (stordriver.Driver, error) {
	return newDriverWithClients(profile, spec, api, presigner)
}

func newDriver(ctx context.Context, profile Profile, spec stordriver.Spec, _ any) (stordriver.Driver, error) {
	if err := validateSpec(profile, &spec); err != nil {
		return nil, err
	}

	cfg, err := loadAWSConfig(ctx, spec.Region, spec)
	if err != nil {
		return nil, fmt.Errorf("%s: load config: %w", profile, err)
	}

	usePathStyle := spec.UsePathStyle || profile == ProfileMinIO
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if spec.Endpoint != "" {
			o.BaseEndpoint = aws.String(spec.Endpoint)
		}
		o.UsePathStyle = usePathStyle
	})
	presignClient := s3.NewPresignClient(client)
	presigner := &awsPresigner{c: presignClient}

	return newDriverWithClients(profile, spec, client, presigner)
}

func newDriverWithClients(profile Profile, spec stordriver.Spec, api API, presigner Presigner) (stordriver.Driver, error) {
	if err := validateSpec(profile, &spec); err != nil {
		return nil, err
	}
	vis := spec.DefaultVisibility
	if !vis.IsValid() {
		vis = stordriver.VisibilityPrivate
	}
	return &impl{
		profile:    profile,
		bucket:     spec.Bucket,
		region:     spec.Region,
		publicURL:  strings.TrimRight(spec.PublicURL, "/"),
		defaultVis: vis,
		api:        api,
		presigner:  presigner,
		uploader:   newUploader(api),
	}, nil
}

// validateSpec enforces required fields and applies profile-specific
// defaults (MinIO region fallback). Called from both code paths so
// NewDriverWithAPI (used by tests) and NewConstructor (used in
// production) share the same checks.
func validateSpec(profile Profile, spec *stordriver.Spec) error {
	if strings.TrimSpace(spec.Bucket) == "" {
		return fmt.Errorf("%s: bucket is required", profile)
	}
	if profile == ProfileMinIO && strings.TrimSpace(spec.Endpoint) == "" {
		return fmt.Errorf("minio: endpoint is required (e.g. http://localhost:9000)")
	}
	if spec.Region == "" && profile == ProfileMinIO {
		spec.Region = "us-east-1"
	}
	if spec.Region == "" {
		return fmt.Errorf("%s: region is required", profile)
	}
	return nil
}

// loadAWSConfig assembles an aws.Config from explicit credentials when
// the Spec carries them, otherwise from the SDK's default chain
// (env vars → shared config → IRSA → EC2 IMDS).
func loadAWSConfig(ctx context.Context, region string, spec stordriver.Spec) (aws.Config, error) {
	opts := []func(*awsconfig.LoadOptions) error{awsconfig.WithRegion(region)}
	if spec.AccessKey != "" && spec.SecretKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			spec.AccessKey, spec.SecretKey, spec.SessionToken,
		)))
	}
	return awsconfig.LoadDefaultConfig(ctx, opts...)
}

// newUploader returns a multipart uploader when api is a real
// *s3.Client; tests using a fake API skip multipart since the fake
// implements PutObject directly.
func newUploader(api API) *manager.Uploader {
	client, ok := api.(*s3.Client)
	if !ok {
		return nil
	}
	return manager.NewUploader(client)
}

// impl is the driver. Safe for concurrent use; the AWS SDK clients are
// goroutine-safe and the uploader manages its own pool.
type impl struct {
	profile    Profile
	bucket     string
	region     string
	publicURL  string
	defaultVis stordriver.Visibility

	api       API
	presigner Presigner
	uploader  *manager.Uploader

	shutdownOnce sync.Once
}

// ── key normalisation ────────────────────────────────────────────────

func cleanKey(key string) (string, error) {
	if strings.TrimSpace(key) == "" {
		return "", fmt.Errorf("s3: empty key")
	}
	cleaned := path.Clean("/" + strings.ReplaceAll(key, `\`, "/"))
	if cleaned == "/" {
		return "", fmt.Errorf("s3: empty key after clean")
	}
	return strings.TrimPrefix(cleaned, "/"), nil
}

// ── Driver ───────────────────────────────────────────────────────────

func (d *impl) NewReader(ctx context.Context, key string) (io.ReadCloser, error) {
	k, err := cleanKey(key)
	if err != nil {
		return nil, err
	}
	out, err := d.api.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &d.bucket,
		Key:    &k,
	})
	if err != nil {
		return nil, translateError(err, key)
	}
	return out.Body, nil
}

func (d *impl) NewWriter(ctx context.Context, key string, opts stordriver.WriteOptions) (io.WriteCloser, error) {
	k, err := cleanKey(key)
	if err != nil {
		return nil, err
	}
	vis := opts.Visibility
	if !vis.IsValid() {
		vis = d.defaultVis
	}
	return newPipeWriter(ctx, d, k, vis, opts), nil
}

func (d *impl) Delete(ctx context.Context, key string) error {
	k, err := cleanKey(key)
	if err != nil {
		return err
	}
	// Pre-flight Exists so we can return ErrNotFound — S3 DeleteObject
	// is idempotent and reports success for missing keys.
	ok, err := d.Exists(ctx, key)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: %s", stordriver.ErrNotFound, key)
	}
	_, err = d.api.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &d.bucket,
		Key:    &k,
	})
	if err != nil {
		return translateError(err, key)
	}
	return nil
}

func (d *impl) Exists(ctx context.Context, key string) (bool, error) {
	k, err := cleanKey(key)
	if err != nil {
		return false, err
	}
	_, err = d.api.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &d.bucket,
		Key:    &k,
	})
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
	out, err := d.api.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &d.bucket,
		Key:    &k,
	})
	if err != nil {
		return stordriver.Attributes{}, translateError(err, key)
	}
	attr := stordriver.Attributes{
		Visibility: stordriver.VisibilityPrivate,
	}
	if out.ContentLength != nil {
		attr.Size = *out.ContentLength
	}
	if out.LastModified != nil {
		attr.LastModified = *out.LastModified
	}
	if out.ContentType != nil {
		attr.ContentType = *out.ContentType
	}
	if out.ETag != nil {
		attr.ETag = strings.Trim(*out.ETag, `"`)
	}
	if len(out.Metadata) > 0 {
		attr.Metadata = make(map[string]string, len(out.Metadata))
		for k, v := range out.Metadata {
			attr.Metadata[k] = v
		}
	}
	return attr, nil
}

func (d *impl) List(ctx context.Context, prefix string) ([]stordriver.Entry, error) {
	// Treat prefix as a directory: strip leading slash, ensure trailing "/".
	p := strings.TrimPrefix(path.Clean("/"+prefix), "/")
	if prefix == "" || prefix == "/" {
		p = ""
	}
	if p != "" && !strings.HasSuffix(p, "/") {
		p += "/"
	}

	in := &s3.ListObjectsV2Input{
		Bucket:    &d.bucket,
		Prefix:    aws.String(p),
		Delimiter: aws.String("/"),
	}
	out, err := d.api.ListObjectsV2(ctx, in)
	if err != nil {
		return nil, translateError(err, prefix)
	}
	entries := make([]stordriver.Entry, 0, len(out.Contents)+len(out.CommonPrefixes))
	for _, obj := range out.Contents {
		if obj.Key == nil {
			continue
		}
		// Skip the placeholder "directory" object some clients create.
		if strings.HasSuffix(*obj.Key, "/") && (obj.Size != nil && *obj.Size == 0) {
			continue
		}
		entry := stordriver.Entry{Key: *obj.Key}
		if obj.Size != nil {
			entry.Size = *obj.Size
		}
		if obj.LastModified != nil {
			entry.LastModified = *obj.LastModified
		}
		entries = append(entries, entry)
	}
	for _, cp := range out.CommonPrefixes {
		if cp.Prefix == nil {
			continue
		}
		key := strings.TrimSuffix(*cp.Prefix, "/")
		entries = append(entries, stordriver.Entry{Key: key, IsDir: true})
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

func (d *impl) SignedURL(ctx context.Context, key string, expires time.Duration) (string, error) {
	if d.presigner == nil {
		return "", fmt.Errorf("%w: presigner not configured", stordriver.ErrNotSupported)
	}
	if expires <= 0 {
		expires = 15 * time.Minute
	}
	k, err := cleanKey(key)
	if err != nil {
		return "", err
	}
	req, err := d.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: &d.bucket,
		Key:    &k,
	}, func(o *s3.PresignOptions) {
		o.Expires = expires
	})
	if err != nil {
		return "", translateError(err, key)
	}
	return req.URL, nil
}

func (d *impl) Shutdown(_ context.Context) error {
	d.shutdownOnce.Do(func() {})
	return nil
}

// ── writer ───────────────────────────────────────────────────────────

type pipeWriter struct {
	pw     *io.PipeWriter
	done   chan error
	closed bool
}

func (w *pipeWriter) Write(p []byte) (int, error) {
	if w.closed {
		return 0, fmt.Errorf("s3: write after close")
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

func newPipeWriter(ctx context.Context, d *impl, key string, vis stordriver.Visibility, opts stordriver.WriteOptions) *pipeWriter {
	pr, pw := io.Pipe()
	done := make(chan error, 1)
	go func() {
		defer pr.Close()
		acl := visibilityToACL(vis)
		var meta map[string]string
		if len(opts.Metadata) > 0 {
			meta = opts.Metadata
		}
		// When uploader is available (real *s3.Client), use multipart to
		// handle arbitrary sizes efficiently. Otherwise (tests), fall
		// back to a buffer-then-PutObject path which the fake API can
		// service directly.
		if d.uploader != nil {
			in := &s3.PutObjectInput{
				Bucket:   &d.bucket,
				Key:      &key,
				Body:     pr,
				ACL:      acl,
				Metadata: meta,
			}
			if opts.ContentType != "" {
				in.ContentType = aws.String(opts.ContentType)
			}
			if opts.CacheControl != "" {
				in.CacheControl = aws.String(opts.CacheControl)
			}
			_, err := d.uploader.Upload(ctx, in)
			done <- err
			return
		}
		// Fallback path for fakes: buffer then PutObject.
		buf, err := io.ReadAll(pr)
		if err != nil {
			done <- err
			return
		}
		in := &s3.PutObjectInput{
			Bucket:   &d.bucket,
			Key:      &key,
			Body:     bytes.NewReader(buf),
			ACL:      acl,
			Metadata: meta,
		}
		if opts.ContentType != "" {
			in.ContentType = aws.String(opts.ContentType)
		}
		if opts.CacheControl != "" {
			in.CacheControl = aws.String(opts.CacheControl)
		}
		_, err = d.api.PutObject(ctx, in)
		done <- err
	}()
	return &pipeWriter{pw: pw, done: done}
}

func visibilityToACL(v stordriver.Visibility) s3types.ObjectCannedACL {
	if v == stordriver.VisibilityPublic {
		return s3types.ObjectCannedACLPublicRead
	}
	return s3types.ObjectCannedACLPrivate
}

// ── presigner adapter ────────────────────────────────────────────────

type awsPresigner struct{ c *s3.PresignClient }

func (a *awsPresigner) PresignGetObject(ctx context.Context, in *s3.GetObjectInput, optFns ...func(*s3.PresignOptions)) (*PresignedRequest, error) {
	req, err := a.c.PresignGetObject(ctx, in, optFns...)
	if err != nil {
		return nil, err
	}
	return &PresignedRequest{URL: req.URL}, nil
}

// ── error translation ────────────────────────────────────────────────

func translateError(err error, key string) error {
	if err == nil {
		return nil
	}
	if isNotFound(err) {
		return fmt.Errorf("%w: %s", stordriver.ErrNotFound, key)
	}
	return fmt.Errorf("s3: %w", err)
}

func isNotFound(err error) bool {
	if err == nil {
		return false
	}
	var nsk *s3types.NoSuchKey
	if errors.As(err, &nsk) {
		return true
	}
	var nf *s3types.NotFound
	if errors.As(err, &nf) {
		return true
	}
	var ae smithy.APIError
	if errors.As(err, &ae) {
		switch ae.ErrorCode() {
		case "NoSuchKey", "NotFound", "404":
			return true
		}
	}
	return false
}
