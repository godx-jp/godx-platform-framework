package validation

import (
	"context"
	"testing"

	"github.com/godx-jp/godx-platform-framework/framework"
)

func TestModuleWiresIntoApp(t *testing.T) {
	app := framework.New("svc", "0.0.0").Use(Module)
	ctx := context.Background()
	if err := app.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()
	v, err := FromApp(app)
	if err != nil {
		t.Fatalf("FromApp: %v", err)
	}
	if v == nil {
		t.Fatalf("nil validator")
	}
	if !v.HasRule("required") {
		t.Fatalf("built-ins missing")
	}
}

func TestModuleWithValidator(t *testing.T) {
	v := New()
	_ = v.AddRule("custom", func(rc RuleContext) error { return nil })
	app := framework.New("svc", "0.0.0").Use(ModuleWithValidator(v))
	ctx := context.Background()
	if err := app.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()
	got, _ := FromApp(app)
	if got != v {
		t.Fatalf("custom validator not wired")
	}
}

func TestModuleNilValidatorRejected(t *testing.T) {
	app := framework.New("svc", "0.0.0").Use(ModuleWithValidator(nil))
	if err := app.Init(context.Background()); err == nil {
		t.Fatalf("expected nil validator err")
	}
}

func TestDuplicateInitRejected(t *testing.T) {
	app := framework.New("svc", "0.0.0").Use(Module).Use(Module)
	if err := app.Init(context.Background()); err == nil {
		t.Fatalf("expected duplicate err")
	}
}

func TestContextHelpers(t *testing.T) {
	v := New()
	ctx := ContextWithValidator(context.Background(), v)
	got, ok := FromContext(ctx)
	if !ok || got != v {
		t.Fatalf("round trip failed")
	}
	if _, ok := FromContext(context.Background()); ok {
		t.Fatalf("plain ctx should miss")
	}
	if _, ok := FromContext(nil); ok {
		t.Fatalf("nil ctx should miss")
	}
}

func TestFromAppMissingModule(t *testing.T) {
	app := framework.New("svc", "0.0.0")
	if err := app.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = app.Shutdown(context.Background()) }()
	if _, err := FromApp(app); err == nil {
		t.Fatalf("expected missing module err")
	}
}
