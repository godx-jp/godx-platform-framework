package s3core_test

import (
	"context"
	"errors"
	"io"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"

	stordriver "github.com/godx-jp/godx-platform-framework/storage/driver"
	"github.com/godx-jp/godx-platform-framework/storage/drivers/internal/s3core"
)

// ── fake S3 API ──────────────────────────────────────────────────────
//
// The fake stores objects in memory keyed by bucket+key. It implements
// every method s3core uses and is goroutine-safe.

type fakeObject struct {
	body         []byte
	contentType  string
	cacheControl string
	acl          s3types.ObjectCannedACL
	metadata     map[string]string
	lastModified time.Time
	etag         string
}

type fakeAPI struct {
	mu      sync.Mutex
	objects map[string]map[string]*fakeObject // bucket → key → object
}

func newFakeAPI() *fakeAPI {
	return &fakeAPI{objects: map[string]map[string]*fakeObject{}}
}

func (f *fakeAPI) bucket(name string) map[string]*fakeObject {
	if _, ok := f.objects[name]; !ok {
		f.objects[name] = map[string]*fakeObject{}
	}
	return f.objects[name]
}

func (f *fakeAPI) GetObject(_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.Options)) (*s3.GetObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.bucket(*in.Bucket)[*in.Key]
	if !ok {
		return nil, &s3types.NoSuchKey{}
	}
	return &s3.GetObjectOutput{
		Body:          io.NopCloser(strings.NewReader(string(o.body))),
		ContentLength: aws.Int64(int64(len(o.body))),
		ContentType:   aws.String(o.contentType),
	}, nil
}

func (f *fakeAPI) PutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.Options)) (*s3.PutObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	body, err := io.ReadAll(in.Body)
	if err != nil {
		return nil, err
	}
	o := &fakeObject{
		body:         body,
		metadata:     in.Metadata,
		lastModified: time.Now().UTC(),
		etag:         "fake-etag",
	}
	if in.ContentType != nil {
		o.contentType = *in.ContentType
	}
	if in.CacheControl != nil {
		o.cacheControl = *in.CacheControl
	}
	o.acl = in.ACL
	f.bucket(*in.Bucket)[*in.Key] = o
	return &s3.PutObjectOutput{ETag: aws.String(`"` + o.etag + `"`)}, nil
}

func (f *fakeAPI) DeleteObject(_ context.Context, in *s3.DeleteObjectInput, _ ...func(*s3.Options)) (*s3.DeleteObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.bucket(*in.Bucket), *in.Key)
	return &s3.DeleteObjectOutput{}, nil
}

func (f *fakeAPI) HeadObject(_ context.Context, in *s3.HeadObjectInput, _ ...func(*s3.Options)) (*s3.HeadObjectOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	o, ok := f.bucket(*in.Bucket)[*in.Key]
	if !ok {
		return nil, &s3types.NotFound{}
	}
	out := &s3.HeadObjectOutput{
		ContentLength: aws.Int64(int64(len(o.body))),
		ContentType:   aws.String(o.contentType),
		LastModified:  &o.lastModified,
		ETag:          aws.String(`"` + o.etag + `"`),
	}
	if len(o.metadata) > 0 {
		out.Metadata = o.metadata
	}
	return out, nil
}

func (f *fakeAPI) ListObjectsV2(_ context.Context, in *s3.ListObjectsV2Input, _ ...func(*s3.Options)) (*s3.ListObjectsV2Output, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	prefix := ""
	if in.Prefix != nil {
		prefix = *in.Prefix
	}
	out := &s3.ListObjectsV2Output{}
	seenDirs := map[string]bool{}
	for k, o := range f.bucket(*in.Bucket) {
		if !strings.HasPrefix(k, prefix) {
			continue
		}
		rest := strings.TrimPrefix(k, prefix)
		if rest == "" {
			continue
		}
		if idx := strings.IndexByte(rest, '/'); idx >= 0 && in.Delimiter != nil && *in.Delimiter == "/" {
			dir := prefix + rest[:idx] + "/"
			if !seenDirs[dir] {
				seenDirs[dir] = true
				out.CommonPrefixes = append(out.CommonPrefixes, s3types.CommonPrefix{Prefix: aws.String(dir)})
			}
			continue
		}
		out.Contents = append(out.Contents, s3types.Object{
			Key:          aws.String(k),
			Size:         aws.Int64(int64(len(o.body))),
			LastModified: &o.lastModified,
		})
	}
	sort.Slice(out.Contents, func(i, j int) bool { return *out.Contents[i].Key < *out.Contents[j].Key })
	sort.Slice(out.CommonPrefixes, func(i, j int) bool { return *out.CommonPrefixes[i].Prefix < *out.CommonPrefixes[j].Prefix })
	return out, nil
}

// ── fake Presigner ───────────────────────────────────────────────────

type fakePresigner struct{ baseURL string }

func (p *fakePresigner) PresignGetObject(_ context.Context, in *s3.GetObjectInput, _ ...func(*s3.PresignOptions)) (*s3core.PresignedRequest, error) {
	return &s3core.PresignedRequest{URL: p.baseURL + "/" + *in.Bucket + "/" + *in.Key + "?X-Amz-Expires=900"}, nil
}

func (p *fakePresigner) PresignPutObject(_ context.Context, in *s3.PutObjectInput, _ ...func(*s3.PresignOptions)) (*s3core.PresignedRequest, error) {
	return &s3core.PresignedRequest{URL: p.baseURL + "/" + *in.Bucket + "/" + *in.Key + "?X-Amz-Expires=900&method=PUT"}, nil
}

// ── tests ────────────────────────────────────────────────────────────

func newDriver(t *testing.T, opts ...func(*stordriver.Spec)) (stordriver.Driver, *fakeAPI) {
	t.Helper()
	api := newFakeAPI()
	spec := stordriver.Spec{
		Bucket:    "test-bucket",
		Region:    "us-east-1",
		PublicURL: "https://cdn.example.com",
	}
	for _, fn := range opts {
		fn(&spec)
	}
	d, err := s3core.NewDriverWithAPI(s3core.ProfileAWS, spec, api, &fakePresigner{baseURL: "https://signed.example.com"})
	if err != nil {
		t.Fatalf("new driver: %v", err)
	}
	t.Cleanup(func() { _ = d.Shutdown(context.Background()) })
	return d, api
}

func TestS3Core_PutGetExistsDeleteRoundTrip(t *testing.T) {
	d, _ := newDriver(t)
	ctx := context.Background()

	w, err := d.NewWriter(ctx, "hello.txt", stordriver.WriteOptions{ContentType: "text/plain"})
	if err != nil {
		t.Fatalf("writer: %v", err)
	}
	_, _ = w.Write([]byte("world"))
	if err := w.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	ok, err := d.Exists(ctx, "hello.txt")
	if err != nil || !ok {
		t.Fatalf("exists: ok=%v err=%v", ok, err)
	}

	r, err := d.NewReader(ctx, "hello.txt")
	if err != nil {
		t.Fatalf("reader: %v", err)
	}
	body, _ := io.ReadAll(r)
	_ = r.Close()
	if string(body) != "world" {
		t.Fatalf("get: want world, got %q", body)
	}

	attr, err := d.Attributes(ctx, "hello.txt")
	if err != nil {
		t.Fatalf("attrs: %v", err)
	}
	if attr.Size != 5 || attr.ContentType != "text/plain" {
		t.Fatalf("attrs unexpected: %+v", attr)
	}

	if err := d.Delete(ctx, "hello.txt"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if ok, _ := d.Exists(ctx, "hello.txt"); ok {
		t.Fatal("expected gone after delete")
	}
}

func TestS3Core_DeleteMissingReturnsNotFound(t *testing.T) {
	d, _ := newDriver(t)
	err := d.Delete(context.Background(), "nope.txt")
	if !errors.Is(err, stordriver.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestS3Core_GetMissingReturnsNotFound(t *testing.T) {
	d, _ := newDriver(t)
	_, err := d.NewReader(context.Background(), "nope.txt")
	if !errors.Is(err, stordriver.ErrNotFound) {
		t.Fatalf("want ErrNotFound, got %v", err)
	}
}

func TestS3Core_WriteVisibilityMapsToACL(t *testing.T) {
	d, api := newDriver(t)
	ctx := context.Background()

	w, _ := d.NewWriter(ctx, "pub.bin", stordriver.WriteOptions{Visibility: stordriver.VisibilityPublic})
	_, _ = w.Write([]byte("x"))
	_ = w.Close()

	api.mu.Lock()
	got := api.bucket("test-bucket")["pub.bin"]
	api.mu.Unlock()
	if got.acl != s3types.ObjectCannedACLPublicRead {
		t.Fatalf("public ACL = %q, want public-read", got.acl)
	}

	w, _ = d.NewWriter(ctx, "priv.bin", stordriver.WriteOptions{Visibility: stordriver.VisibilityPrivate})
	_, _ = w.Write([]byte("y"))
	_ = w.Close()
	api.mu.Lock()
	got = api.bucket("test-bucket")["priv.bin"]
	api.mu.Unlock()
	if got.acl != s3types.ObjectCannedACLPrivate {
		t.Fatalf("private ACL = %q, want private", got.acl)
	}
}

func TestS3Core_WriteOptionsForwardedToPutObject(t *testing.T) {
	d, api := newDriver(t)
	ctx := context.Background()

	w, _ := d.NewWriter(ctx, "img.jpg", stordriver.WriteOptions{
		ContentType:  "image/jpeg",
		CacheControl: "max-age=3600",
		Metadata:     map[string]string{"author": "alice"},
	})
	_, _ = w.Write([]byte("body"))
	_ = w.Close()

	api.mu.Lock()
	o := api.bucket("test-bucket")["img.jpg"]
	api.mu.Unlock()
	if o.contentType != "image/jpeg" || o.cacheControl != "max-age=3600" {
		t.Fatalf("write options not forwarded: ct=%q cc=%q", o.contentType, o.cacheControl)
	}
	if o.metadata["author"] != "alice" {
		t.Fatalf("metadata not forwarded: %v", o.metadata)
	}
}

func TestS3Core_ListGroupsByDirectory(t *testing.T) {
	d, _ := newDriver(t)
	ctx := context.Background()

	keys := []string{"a.txt", "b.txt", "sub/c.txt", "sub/deep/d.txt", "sub/e.txt"}
	for _, k := range keys {
		w, _ := d.NewWriter(ctx, k, stordriver.WriteOptions{})
		_, _ = w.Write([]byte("x"))
		_ = w.Close()
	}

	root, err := d.List(ctx, "")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	var files, dirs []string
	for _, e := range root {
		if e.IsDir {
			dirs = append(dirs, e.Key)
		} else {
			files = append(files, e.Key)
		}
	}
	sort.Strings(files)
	sort.Strings(dirs)
	if len(files) != 2 || files[0] != "a.txt" || files[1] != "b.txt" {
		t.Fatalf("root files = %v", files)
	}
	if len(dirs) != 1 || dirs[0] != "sub" {
		t.Fatalf("root dirs = %v", dirs)
	}

	sub, _ := d.List(ctx, "sub")
	files, dirs = nil, nil
	for _, e := range sub {
		if e.IsDir {
			dirs = append(dirs, e.Key)
		} else {
			files = append(files, e.Key)
		}
	}
	sort.Strings(files)
	sort.Strings(dirs)
	if len(files) != 2 || files[0] != "sub/c.txt" || files[1] != "sub/e.txt" {
		t.Fatalf("sub files = %v", files)
	}
	if len(dirs) != 1 || dirs[0] != "sub/deep" {
		t.Fatalf("sub dirs = %v", dirs)
	}
}

func TestS3Core_URLRequiresPublicURL(t *testing.T) {
	d, _ := newDriver(t, func(s *stordriver.Spec) { s.PublicURL = "" })
	if _, err := d.URL("x"); !errors.Is(err, stordriver.ErrNotSupported) {
		t.Fatalf("expected ErrNotSupported, got %v", err)
	}

	d2, _ := newDriver(t)
	u, err := d2.URL("avatars/1.jpg")
	if err != nil {
		t.Fatalf("url: %v", err)
	}
	if u != "https://cdn.example.com/avatars/1.jpg" {
		t.Fatalf("URL = %q", u)
	}
}

func TestS3Core_SignedURLDefaultExpiry(t *testing.T) {
	d, _ := newDriver(t)
	u, err := d.SignedURL(context.Background(), "secret.bin", 0)
	if err != nil {
		t.Fatalf("signed: %v", err)
	}
	if !strings.HasPrefix(u, "https://signed.example.com/test-bucket/secret.bin") {
		t.Fatalf("signed URL = %q", u)
	}
}

func TestS3Core_SignedPutURLDefaultExpiry(t *testing.T) {
	d, _ := newDriver(t)
	u, err := d.SignedPutURL(context.Background(), "upload.bin", 0)
	if err != nil {
		t.Fatalf("signed put: %v", err)
	}
	if !strings.Contains(u, "test-bucket/upload.bin") {
		t.Fatalf("signed put URL = %q", u)
	}
}

func TestS3Core_BucketRequired(t *testing.T) {
	_, err := s3core.NewDriverWithAPI(s3core.ProfileAWS, stordriver.Spec{Region: "us-east-1"}, newFakeAPI(), &fakePresigner{})
	if err == nil || !strings.Contains(err.Error(), "bucket") {
		t.Fatalf("want bucket-required, got %v", err)
	}
}

// Through NewConstructor (vs NewDriverWithAPI) we exercise the AWS
// config-loading path with the MinIO profile: endpoint is required.
func TestS3Core_NewConstructor_MinIOEndpointRequired(t *testing.T) {
	ctor := s3core.NewConstructor(s3core.ProfileMinIO)
	_, err := ctor(context.Background(), stordriver.Spec{Bucket: "b"})
	if err == nil || !strings.Contains(err.Error(), "endpoint is required") {
		t.Fatalf("want endpoint-required, got %v", err)
	}
}

// Through NewConstructor with AWS profile, region is required when not
// supplied.
func TestS3Core_NewConstructor_AWSRegionRequired(t *testing.T) {
	ctor := s3core.NewConstructor(s3core.ProfileAWS)
	_, err := ctor(context.Background(), stordriver.Spec{Bucket: "b"})
	if err == nil || !strings.Contains(err.Error(), "region is required") {
		t.Fatalf("want region-required, got %v", err)
	}
}
