package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/godx-jp/godx-platform-framework/framework"
)

func TestHealthz(t *testing.T) {
	rec := httptest.NewRecorder()
	Healthz(rec, httptest.NewRequest(http.MethodGet, PathHealthz, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestReadyzAllPass(t *testing.T) {
	reg := NewRegistry()
	reg.RegisterProbe("cache", func(ctx context.Context) error { return nil })
	rec := httptest.NewRecorder()
	Readyz(reg, Options{})(rec, httptest.NewRequest(http.MethodGet, PathReadyz, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("code=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestReadyzFailure(t *testing.T) {
	reg := NewRegistry()
	reg.RegisterProbe("db", func(ctx context.Context) error {
		return errors.New("connection refused")
	})
	rec := httptest.NewRecorder()
	Readyz(reg, Options{})(rec, httptest.NewRequest(http.MethodGet, PathReadyz, nil))
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("code=%d", rec.Code)
	}
}

func TestModuleWiresRegistry(t *testing.T) {
	app := framework.New("test", "0").Use(Module)
	if err := app.Init(context.Background()); err != nil {
		t.Fatal(err)
	}
	reg, err := FromApp(app)
	if err != nil {
		t.Fatal(err)
	}
	reg.RegisterProbe("self", func(ctx context.Context) error { return nil })
	if len(reg.Probes()) != 1 {
		t.Fatalf("probes=%v", reg.Probes())
	}
}

func TestRegisterProbeOverwrite(t *testing.T) {
	reg := NewRegistry()
	reg.RegisterProbe("x", func(ctx context.Context) error { return nil })
	reg.RegisterProbe("x", func(ctx context.Context) error { return errors.New("fail") })
	failures := reg.CheckReady(context.Background())
	if len(failures) != 1 {
		t.Fatalf("failures=%v", failures)
	}
}
