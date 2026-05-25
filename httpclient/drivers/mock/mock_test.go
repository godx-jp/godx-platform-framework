package mock

import (
	"context"
	"io"
	"net/http"
	"testing"
)

func TestMockRecordsAndResponds(t *testing.T) {
	c := New()
	c.PushResponse(200, []byte("hello"), nil)
	req, _ := http.NewRequest(http.MethodGet, "http://example/", nil)
	resp, err := c.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if string(b) != "hello" {
		t.Fatalf("body=%q", b)
	}
	if len(c.Requests()) != 1 {
		t.Fatalf("requests=%d", len(c.Requests()))
	}
}
