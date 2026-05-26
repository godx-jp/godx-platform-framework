package database

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestLoadConfigFromEnvDefaults(t *testing.T) {
	t.Setenv(EnvDefaultConnection, "")
	t.Setenv(EnvConnections, "")
	t.Setenv("DATABASE_URL", "postgres://localhost/db")

	cfg := LoadConfigFromEnv()
	if cfg.DefaultConnection != "default" {
		t.Fatalf("default=%q", cfg.DefaultConnection)
	}
	if _, ok := cfg.Connections["default"]; !ok {
		t.Fatal("missing default connection")
	}
	if cfg.Connections["default"].URL != "postgres://localhost/db" {
		t.Fatalf("url=%q", cfg.Connections["default"].URL)
	}
	if cfg.ReadStrategy != ReadRoundRobin {
		t.Fatalf("strategy=%q", cfg.ReadStrategy)
	}
	if !cfg.Sticky {
		t.Fatal("expected sticky default true")
	}
}

func TestConfigValidate(t *testing.T) {
	cfg := Config{
		DefaultConnection: "default",
		WriteConnection:   "default",
		Connections: map[string]ConnectionConfig{
			"default": {Driver: "postgres", URL: "postgres://x/y"},
		},
		MetricsInterval: 15 * time.Second,
	}
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestManagerReadWriteSticky(t *testing.T) {
	mgr := NewManager()
	mgr.configureRouting(Config{
		WriteConnection: "write",
		ReadConnections: []string{"read"},
		Sticky:          true,
	})
	write := newConnection("write", &fakeHandle{name: "postgresql"})
	read := newConnection("read", &fakeHandle{name: "postgresql"})
	_ = mgr.AddConnection(write)
	_ = mgr.AddConnection(read)

	got, err := mgr.Read(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if got.Name() != "read" {
		t.Fatalf("read=%q", got.Name())
	}

	ctx := MarkWritten(t.Context())
	got, err = mgr.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name() != "write" {
		t.Fatalf("sticky read=%q", got.Name())
	}
}

type fakeHandle struct{ name string }

func (f *fakeHandle) Ping(context.Context) error     { return nil }
func (f *fakeHandle) Shutdown(context.Context) error { return nil }
func (f *fakeHandle) Postgres() *pgxpool.Pool        { return nil }
func (f *fakeHandle) SQL() *sql.DB                   { return nil }
func (f *fakeHandle) System() string                 { return f.name }
func (f *fakeHandle) DriverName() string             { return "postgres" }
