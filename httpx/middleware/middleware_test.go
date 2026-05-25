package middleware

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/godx-jp/godx-platform-framework/httpx"
	_ "github.com/godx-jp/godx-platform-framework/ratelimit"
	rdriver "github.com/godx-jp/godx-platform-framework/ratelimit/driver"
	"github.com/godx-jp/godx-platform-framework/validation"
)

func TestPipelineMiddleware(t *testing.T) {
	var hit bool
	logStage := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hit = true
			next.ServeHTTP(w, r)
		})
	}
	h := Pipeline(logStage)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if !hit {
		t.Fatal("pipeline stage not run")
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestValidateJSON(t *testing.T) {
	type body struct {
		Email string `json:"email" validate:"required,email"`
	}
	v := validation.New()
	mw := ValidateJSON(v, func() any { return &body{} })
	called := false
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dto, ok := Validated[*body](r)
		if !ok || dto.Email != "a@b.co" {
			t.Fatalf("dto=%v ok=%v", dto, ok)
		}
		called = true
		w.WriteHeader(http.StatusOK)
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"email":"a@b.co"}`))
	req.Header.Set("Content-Type", "application/json")
	h.ServeHTTP(rec, req)
	if !called {
		t.Fatal("handler not called")
	}
}

func TestValidateJSONRejects(t *testing.T) {
	type body struct {
		Email string `json:"email" validate:"required,email"`
	}
	v := validation.New()
	mw := ValidateJSON(v, func() any { return &body{} })
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"email":"bad"}`))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestValidateJSONStructuredErrorNoLeak(t *testing.T) {
	type body struct {
		Email string `json:"email" validate:"required,email"`
	}
	v := validation.New()
	mw := ValidateJSON(v, func() any { return &body{} })
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"email":"bad"}`))
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("code=%d", rec.Code)
	}
	var resp struct {
		Error  string `json:"error"`
		Fields []struct {
			Field string `json:"field"`
			Code  string `json:"code"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("response not JSON: %v body=%s", err, rec.Body.String())
	}
	if len(resp.Fields) == 0 {
		t.Fatalf("expected structured field errors, got %s", rec.Body.String())
	}
	if resp.Fields[0].Code == "" {
		t.Fatalf("expected machine code (rule), got %s", rec.Body.String())
	}
	// Must not leak the raw validator/Go error text. The raw aggregate
	// error string (validation.Errors.Error()) joins per-field messages
	// like "Email: email failed"; the structured response must not contain
	// that text nor the "field: message" joiner syntax.
	bodyStr := rec.Body.String()
	rawErr := v.ValidateStruct(req.Context(), &body{Email: "bad"}).Error()
	if strings.Contains(bodyStr, rawErr) || strings.Contains(bodyStr, "Email:") {
		t.Fatalf("response leaks raw validator error %q: %s", rawErr, bodyStr)
	}
}

func TestValidateJSONOversizedBody(t *testing.T) {
	type body struct {
		Email string `json:"email" validate:"required,email"`
	}
	v := validation.New()
	mw := ValidateJSON(v, func() any { return &body{} })
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))
	big := `{"email":"` + strings.Repeat("a", int(httpx.DefaultMaxBodyBytes)+1024) + `"}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(big))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("code=%d want 413", rec.Code)
	}
}

func TestValidateJSONMalformedBody(t *testing.T) {
	type body struct {
		Email string `json:"email" validate:"required,email"`
	}
	v := validation.New()
	mw := ValidateJSON(v, func() any { return &body{} })
	h := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("should not reach handler")
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(`{"email":`))
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("code=%d want 400", rec.Code)
	}
}

func TestRateLimitByIP(t *testing.T) {
	lim, err := rdriver.New(context.Background(), rdriver.Spec{Name: rdriver.DriverMemory, Rate: 1, Burst: 1})
	if err != nil {
		t.Fatal(err)
	}
	h := RateLimitByIP(lim)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("first code=%d", rec.Code)
	}
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, req)
	if rec2.Code != http.StatusTooManyRequests {
		t.Fatalf("second code=%d", rec2.Code)
	}
}
