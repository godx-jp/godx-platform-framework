package httpx

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

const (
	// ProblemContentType is the RFC 9457 Section 3 mandatory Content-Type for Problem JSON.
	ProblemContentType = "application/problem+json; charset=utf-8"

	// ContentType is an alias for ProblemContentType.
	ContentType = ProblemContentType
)

// FieldError describes one validation failure inside a 422 Problem response.
type FieldError struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// Problem implements RFC 9457 Problem Details for HTTP APIs.
// Required: Type, Title, Status. Detail/Instance/Code/RequestID/Errors are optional.
type Problem struct {
	Type      string       `json:"type"`
	Title     string       `json:"title"`
	Status    int          `json:"status"`
	Detail    string       `json:"detail,omitempty"`
	Instance  string       `json:"instance,omitempty"`
	Code      string       `json:"code,omitempty"`
	RequestID string       `json:"request_id,omitempty"`
	Errors    []FieldError `json:"errors,omitempty"`
}

// ProblemOpt mutates a Problem before write.
type ProblemOpt func(*Problem)

// WithErrors attaches per-field validation errors (typically with status 422).
func WithErrors(errs ...FieldError) ProblemOpt {
	return func(p *Problem) { p.Errors = append(p.Errors, errs...) }
}

// WithCode overrides the machine code (defaults to slug).
func WithCode(code string) ProblemOpt {
	return func(p *Problem) { p.Code = code }
}

// WithInstance sets the instance URI; if empty, RequestID is reused.
func WithInstance(instance string) ProblemOpt {
	return func(p *Problem) { p.Instance = instance }
}

// titles maps slug to static EN title. Extend at runtime via RegisterProblemTitle.
var titles = map[string]string{
	"validation-failed":       "Validation failed",
	"tenancy-missing":         "Tenancy context missing",
	"missing-idempotency-key": "Idempotency-Key header required",
	"idempotency-mismatch":    "Idempotency-Key body mismatch",
	"unauthenticated":         "Unauthenticated",
	"forbidden":               "Forbidden",
	"not-found":               "Not found",
	"conflict":                "Conflict",
	"rate-limit":              "Too many requests",
	"internal-error":          "Internal server error",
	"unavailable":             "Service unavailable",
	"bad-request":             "Bad request",
}

var problemTypeBaseURL string

// SetProblemTypeBaseURL sets the prefix for RFC 9457 type URIs built by TypeURI
// and WriteProblem. Pass the scheme + host only (no trailing slash), e.g.
// "https://errors.example.com". When unset, TypeURI falls back to
// "urn:problem:{service}:{slug}".
func SetProblemTypeBaseURL(base string) {
	problemTypeBaseURL = strings.TrimRight(base, "/")
}

// RegisterProblemTitle registers a human-readable title for a problem slug.
func RegisterProblemTitle(slug, title string) {
	if slug == "" || title == "" {
		return
	}
	titles[slug] = title
}

// Title returns the static EN title for slug; falls back to slug if unknown.
func Title(slug string) string {
	if t, ok := titles[slug]; ok {
		return t
	}
	return slug
}

// TypeURI builds the RFC 9457 type URI for service and slug.
// service is typically the deploying service name (kebab-case).
func TypeURI(service, slug string) string {
	if problemTypeBaseURL != "" {
		return fmt.Sprintf("%s/%s/%s", problemTypeBaseURL, service, slug)
	}
	return fmt.Sprintf("urn:problem:%s:%s", service, slug)
}

// RequestIDExtractor pulls a correlation ID off the request context.
type RequestIDExtractor func(context.Context) string

var requestIDExtractor RequestIDExtractor = func(context.Context) string { return "" }

// SetRequestIDExtractor configures how WriteProblem looks up request_id.
func SetRequestIDExtractor(fn RequestIDExtractor) {
	if fn != nil {
		requestIDExtractor = fn
	}
}

// WriteProblem serializes a Problem and writes it to w.
//
// service: kebab-case service identifier used in the type URI.
// status:  HTTP status code (4xx / 5xx).
// slug:    kebab-case error slug, used as both code and last segment of type.
// detail:  human-readable message.
// opts:    optional WithErrors / WithCode / WithInstance.
func WriteProblem(w http.ResponseWriter, r *http.Request, service string, status int, slug, detail string, opts ...ProblemOpt) {
	p := &Problem{
		Type:   TypeURI(service, slug),
		Title:  Title(slug),
		Status: status,
		Detail: detail,
		Code:   slug,
	}
	if r != nil {
		p.RequestID = requestIDExtractor(r.Context())
		if p.Instance == "" {
			p.Instance = p.RequestID
		}
	}
	for _, o := range opts {
		o(p)
	}
	w.Header().Set("Content-Type", ContentType)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(p)
}
