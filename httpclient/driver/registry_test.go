package driver

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

type fakeClient struct{}

func (fakeClient) Name() string                            { return "fake" }
func (fakeClient) Do(context.Context, *http.Request) (*http.Response, error) {
	return nil, nil
}
func (fakeClient) Shutdown(context.Context) error { return nil }

func TestRegisterLookupNew(t *testing.T) {
	Register("fake_hc_test", func(ctx context.Context, spec Spec) (Client, error) {
		return fakeClient{}, nil
	})
	defer func() {
		regMu.Lock()
		delete(reg, "fake_hc_test")
		regMu.Unlock()
	}()
	c, err := New(context.Background(), Spec{Name: "fake_hc_test"})
	if err != nil || c == nil {
		t.Fatalf("New: %v", err)
	}
}

func TestNewEmptyName(t *testing.T) {
	if _, err := New(context.Background(), Spec{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRegisterPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	Register("", nil)
}

func TestSentinelDistinct(t *testing.T) {
	es := []error{ErrClosed, ErrCircuitOpen, ErrInvalidBaseURL}
	for i, a := range es {
		for j, b := range es {
			if i != j && errors.Is(a, b) {
				t.Fatalf("collide")
			}
		}
	}
}
