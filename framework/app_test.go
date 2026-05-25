package framework_test

import (
	"context"
	"errors"
	"testing"

	"github.com/godx-jp/godx-platform-framework/framework"
)

type recorderModule struct {
	name        string
	initErr     error
	shutdownErr error
	initOrder   *[]string
	shutOrder   *[]string
}

func (m *recorderModule) Name() string { return m.name }

func (m *recorderModule) Init(_ context.Context, app *framework.App) error {
	if m.initErr != nil {
		return m.initErr
	}
	*m.initOrder = append(*m.initOrder, m.name)
	app.Store(m.name+".value", "set-by-"+m.name)
	app.OnShutdown(func(_ context.Context) error {
		*m.shutOrder = append(*m.shutOrder, m.name)
		return m.shutdownErr
	})
	return nil
}

func TestApp_Init_OrderAndStore(t *testing.T) {
	t.Parallel()

	var initOrder, shutOrder []string
	a := framework.New("svc", "1.2.3").
		Use(&recorderModule{name: "first", initOrder: &initOrder, shutOrder: &shutOrder}).
		Use(&recorderModule{name: "second", initOrder: &initOrder, shutOrder: &shutOrder})

	if err := a.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if got, want := initOrder, []string{"first", "second"}; !equalSlices(got, want) {
		t.Fatalf("init order = %v, want %v", got, want)
	}

	if v, ok := a.Lookup("first.value"); !ok || v != "set-by-first" {
		t.Fatalf("Lookup(first.value) = %v, %v", v, ok)
	}
	if a.Name() != "svc" || a.Version() != "1.2.3" {
		t.Fatalf("Name/Version mismatch: %s %s", a.Name(), a.Version())
	}
}

func TestApp_Init_IsIdempotent(t *testing.T) {
	t.Parallel()

	var initOrder, shutOrder []string
	a := framework.New("svc", "1.0.0").
		Use(&recorderModule{name: "m", initOrder: &initOrder, shutOrder: &shutOrder})

	for i := 0; i < 3; i++ {
		if err := a.Init(context.Background()); err != nil {
			t.Fatalf("Init iter %d: %v", i, err)
		}
	}
	if len(initOrder) != 1 {
		t.Fatalf("module init ran %d times, want 1", len(initOrder))
	}
}

func TestApp_Init_PropagatesModuleError(t *testing.T) {
	t.Parallel()

	bang := errors.New("boom")
	var initOrder, shutOrder []string
	a := framework.New("svc", "1.0.0").
		Use(&recorderModule{name: "ok", initOrder: &initOrder, shutOrder: &shutOrder}).
		Use(&recorderModule{name: "fail", initErr: bang, initOrder: &initOrder, shutOrder: &shutOrder})

	err := a.Init(context.Background())
	if err == nil || !errors.Is(err, bang) {
		t.Fatalf("Init err = %v, want wrapping %v", err, bang)
	}
}

func TestApp_Shutdown_ReverseOrder(t *testing.T) {
	t.Parallel()

	var initOrder, shutOrder []string
	a := framework.New("svc", "1.0.0").
		Use(&recorderModule{name: "a", initOrder: &initOrder, shutOrder: &shutOrder}).
		Use(&recorderModule{name: "b", initOrder: &initOrder, shutOrder: &shutOrder}).
		Use(&recorderModule{name: "c", initOrder: &initOrder, shutOrder: &shutOrder})

	if err := a.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	if err := a.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}

	if got, want := shutOrder, []string{"c", "b", "a"}; !equalSlices(got, want) {
		t.Fatalf("shutdown order = %v, want %v", got, want)
	}
}

func TestApp_Shutdown_JoinsErrors(t *testing.T) {
	t.Parallel()

	e1 := errors.New("e1")
	e2 := errors.New("e2")
	var initOrder, shutOrder []string
	a := framework.New("svc", "1.0.0").
		Use(&recorderModule{name: "a", shutdownErr: e1, initOrder: &initOrder, shutOrder: &shutOrder}).
		Use(&recorderModule{name: "b", shutdownErr: e2, initOrder: &initOrder, shutOrder: &shutOrder})

	if err := a.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	err := a.Shutdown(context.Background())
	if err == nil || !errors.Is(err, e1) || !errors.Is(err, e2) {
		t.Fatalf("Shutdown err = %v, want to wrap both %v and %v", err, e1, e2)
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
