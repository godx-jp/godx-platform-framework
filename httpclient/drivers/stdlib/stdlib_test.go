package stdlib

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	hdriver "github.com/godx-jp/godx-platform-framework/httpclient/driver"
)

func TestRoundTrip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, "ok")
	}))
	defer srv.Close()

	c, err := New(hdriver.Spec{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer c.Shutdown(context.Background())

	req, _ := http.NewRequest(http.MethodGet, "/test", nil)
	resp, err := c.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "ok" {
		t.Fatalf("body=%q", b)
	}
}
