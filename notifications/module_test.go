package notifications

import (
	"context"
	"sync"
	"testing"

	"github.com/godx-jp/godx-platform-framework/framework"
	mdriver "github.com/godx-jp/godx-platform-framework/mail/driver"
	"github.com/godx-jp/godx-platform-framework/mail"
	ndriver "github.com/godx-jp/godx-platform-framework/notifications/driver"
)

type testUser struct {
	email string
}

func (u testUser) RouteNotificationFor(channel string) string {
	if channel == ndriver.DriverMail || channel == "mail" {
		return u.email
	}
	return ""
}

type welcomeNotification struct{}

func (welcomeNotification) Via(Notifiable) []string { return []string{"log"} }

type fakeChannel struct {
	sent int
	mu   sync.Mutex
}

func (f *fakeChannel) Name() string { return "fake" }
func (f *fakeChannel) Send(_ context.Context, _, _ any) error {
	f.mu.Lock()
	f.sent++
	f.mu.Unlock()
	return nil
}
func (f *fakeChannel) Shutdown(context.Context) error { return nil }

func TestManagerSend(t *testing.T) {
	fc := &fakeChannel{}
	mgr := NewManager()
	_ = mgr.AddChannel("log", fc)
	if err := mgr.Send(context.Background(), testUser{email: "a@b.c"}, welcomeNotification{}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if fc.sent != 1 {
		t.Fatalf("sent=%d", fc.sent)
	}
}

func TestModuleWiresLogChannel(t *testing.T) {
	cfg := Config{
		Default: "log",
		Channels: map[string]ChannelConfig{
			"log": {Driver: ndriver.DriverLog, Spec: ndriver.Spec{Name: ndriver.DriverLog}},
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
	if err := mgr.Send(ctx, testUser{email: "u@example.com"}, welcomeNotification{}); err != nil {
		t.Fatalf("Send: %v", err)
	}
}

func TestModuleWithMailChannel(t *testing.T) {
	mailCfg := mail.Config{
		Default: "primary",
		From:    "noreply@example.com",
		Mailers: map[string]mail.MailerConfig{
			"primary": {Driver: mdriver.DriverLog, Spec: mdriver.Spec{Name: mdriver.DriverLog}},
		},
	}
	notifyCfg := Config{
		Default: "log",
		Channels: map[string]ChannelConfig{
			"log":  {Driver: ndriver.DriverLog, Spec: ndriver.Spec{Name: ndriver.DriverLog}},
			"mail": {Driver: ndriver.DriverMail, Spec: ndriver.Spec{Name: ndriver.DriverMail}},
		},
	}
	app := framework.New("svc", "0.0.0").
		Use(mail.ModuleWithConfig(mailCfg)).
		Use(ModuleWithConfig(notifyCfg))
	ctx := context.Background()
	if err := app.Init(ctx); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer func() { _ = app.Shutdown(ctx) }()
	mgr, _ := FromApp(app)
	if err := mgr.Send(ctx, testUser{email: "user@example.com"}, mailWelcome{}); err != nil {
		t.Fatalf("Send: %v", err)
	}
}

type mailWelcome struct{}

func (mailWelcome) Via(Notifiable) []string { return []string{"mail"} }
func (mailWelcome) ToMail(Notifiable) MailMessage {
	return MailMessage{Subject: "Welcome", Body: "Hello"}
}

type memDB struct {
	mu   sync.Mutex
	rows []ndriver.DatabaseRecord
}

func (m *memDB) Store(_ context.Context, rec ndriver.DatabaseRecord) error {
	m.mu.Lock()
	m.rows = append(m.rows, rec)
	m.mu.Unlock()
	return nil
}

type dbUser struct {
	testUser
	typ, id string
}

func (u dbUser) NotifiableType() string { return u.typ }
func (u dbUser) NotifiableID() string   { return u.id }

type dbNote struct{}

func (dbNote) Via(Notifiable) []string { return []string{"database"} }
func (dbNote) ToDatabase(Notifiable) DatabaseMessage {
	return DatabaseMessage{Type: "welcome", Data: []byte(`{"ok":true}`)}
}

func TestDatabaseChannel(t *testing.T) {
	store := &memDB{}
	mgr := NewManager()
	ch, err := buildChannel(context.Background(), ChannelConfig{
		Driver: ndriver.DriverDatabase,
		Spec:   ndriver.Spec{Name: ndriver.DriverDatabase},
	}, buildDeps{databaseStore: store})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	_ = mgr.AddChannel("database", ch)
	u := dbUser{testUser: testUser{email: "a@b.c"}, typ: "user", id: "1"}
	if err := mgr.Send(context.Background(), u, dbNote{}); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if len(store.rows) != 1 {
		t.Fatalf("rows=%d", len(store.rows))
	}
}

func TestValidateRejectsDatabaseWithoutStore(t *testing.T) {
	cfg := Config{
		Default: "database",
		Channels: map[string]ChannelConfig{
			"database": {Driver: ndriver.DriverDatabase},
		},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected error")
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
