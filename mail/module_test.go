package mail

import (
	"context"
	"sync"
	"testing"

	"github.com/godx-jp/godx-platform-framework/events"
	"github.com/godx-jp/godx-platform-framework/framework"
	mdriver "github.com/godx-jp/godx-platform-framework/mail/driver"
)

type fakeTransport struct {
	name string
	sent []mdriver.Message
	mu   sync.Mutex
}

func (f *fakeTransport) Name() string { return f.name }
func (f *fakeTransport) Send(_ context.Context, msg mdriver.Message) error {
	f.mu.Lock()
	f.sent = append(f.sent, msg)
	f.mu.Unlock()
	return nil
}
func (f *fakeTransport) Shutdown(context.Context) error { return nil }

func TestManagerMailerSend(t *testing.T) {
	ft := &fakeTransport{name: "fake"}
	mgr := NewManager()
	if err := mgr.AddTransport("primary", ft); err != nil {
		t.Fatalf("AddTransport: %v", err)
	}
	ml, err := mgr.Mailer()
	if err != nil {
		t.Fatalf("Mailer: %v", err)
	}
	if err := ml.To("user@example.com").Subject("Hi").Body("Hello").Send(context.Background()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(ft.sent) != 1 || ft.sent[0].Subject != "Hi" {
		t.Fatalf("sent=%+v", ft.sent)
	}
}

func TestManagerEmitsEvents(t *testing.T) {
	ft := &fakeTransport{name: "fake"}
	bus := events.New()
	var seen []string
	bus.Listen("mail.*", func(_ context.Context, e events.Event) error {
		seen = append(seen, e.Name)
		return nil
	})
	mgr := NewManager()
	mgr.SetBus(bus)
	_ = mgr.AddTransport("primary", ft)
	ml, _ := mgr.Mailer()
	_ = ml.To("a@b.c").Subject("S").Body("B").Send(context.Background())
	if len(seen) < 2 {
		t.Fatalf("events=%v", seen)
	}
}

func TestModuleWiresIntoApp(t *testing.T) {
	cfg := Config{
		Default: "primary",
		From:    "noreply@example.com",
		Mailers: map[string]MailerConfig{
			"primary": {Driver: mdriver.DriverLog, Spec: mdriver.Spec{Name: mdriver.DriverLog}},
		},
	}
	app := framework.New("svc", "0.0.0").Use(ModuleWithConfig(cfg))
	ctx := context.Background()
	if err := app.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()
	mgr, err := FromApp(app)
	if err != nil {
		t.Fatalf("FromApp: %v", err)
	}
	ml, err := mgr.Mailer()
	if err != nil {
		t.Fatalf("Mailer: %v", err)
	}
	if err := ml.To("user@example.com").Subject("Test").Body("Hi").Send(ctx); err != nil {
		t.Fatalf("Send: %v", err)
	}
}

func TestModuleDuplicateInitRejected(t *testing.T) {
	cfg := Config{
		Default: "x",
		Mailers: map[string]MailerConfig{"x": {Driver: mdriver.DriverLog, Spec: mdriver.Spec{Name: mdriver.DriverLog}}},
	}
	app := framework.New("svc", "0.0.0").Use(ModuleWithConfig(cfg)).Use(ModuleWithConfig(cfg))
	if err := app.Init(context.Background()); err == nil {
		t.Fatal("expected duplicate init error")
	}
}

func TestValidateRejects(t *testing.T) {
	for _, tc := range []struct {
		name string
		cfg  Config
	}{
		{"empty default", Config{Mailers: map[string]MailerConfig{"x": {Driver: "log"}}}},
		{"no mailers", Config{Default: "x"}},
		{"default missing", Config{Default: "x", Mailers: map[string]MailerConfig{"y": {Driver: "log"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.cfg.Validate(); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

func TestFromAppMissingModule(t *testing.T) {
	app := framework.New("svc", "0.0.0")
	_ = app.Init(context.Background())
	defer func() { _ = app.Shutdown(context.Background()) }()
	if _, err := FromApp(app); err == nil {
		t.Fatal("expected error")
	}
}
