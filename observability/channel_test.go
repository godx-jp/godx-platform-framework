package observability_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/godx-jp/godx-platform-framework/framework"
	"github.com/godx-jp/godx-platform-framework/observability"
)

func TestChannel_RoutesToOwnDriver(t *testing.T) {
	dir := t.TempDir()
	auditPath := filepath.Join(dir, "audit.log")

	_ = captureStdout(t, func() {
		t.Setenv("OBSERVABILITY_DRIVER", "stdout")

		app := framework.New("svc", "1.0.0").
			Use(observability.Module).
			Use(observability.NewChannel("audit", observability.Config{
				Driver:      observability.DriverFile,
				LogFilePath: auditPath,
			}))

		if err := app.Init(context.Background()); err != nil {
			t.Fatalf("Init: %v", err)
		}
		defer app.Shutdown(context.Background())

		obs := observability.FromApp(app)
		obs.Channel("audit").Info("user-logged-in", "user_id", 42)
	})

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("ReadFile audit: %v", err)
	}
	var rec map[string]any
	if err := json.Unmarshal([]byte(firstLine(string(data))), &rec); err != nil {
		t.Fatalf("decode audit record: %v\nraw=%q", err, string(data))
	}
	if rec["msg"] != "user-logged-in" || rec["user_id"] != float64(42) {
		t.Fatalf("audit record wrong: %+v", rec)
	}
}

func TestChannel_PrimaryReservedName(t *testing.T) {
	_ = captureStdout(t, func() {
		t.Setenv("OBSERVABILITY_DRIVER", "stdout")

		app := framework.New("svc", "1.0.0").
			Use(observability.Module).
			Use(observability.NewChannel(observability.PrimaryChannel, observability.Config{
				Driver: observability.DriverStdout,
			}))

		err := app.Init(context.Background())
		if err == nil {
			t.Fatalf("expected error registering reserved 'primary' channel")
		}
		if !strings.Contains(err.Error(), "reserved") {
			t.Fatalf("err should mention 'reserved': %v", err)
		}
	})
}

func TestChannel_UnknownChannelFallsBackToPrimary(t *testing.T) {
	out := captureStdout(t, func() {
		t.Setenv("OBSERVABILITY_DRIVER", "stdout")

		app := framework.New("svc", "1.0.0").Use(observability.Module)
		if err := app.Init(context.Background()); err != nil {
			t.Fatalf("Init: %v", err)
		}
		defer app.Shutdown(context.Background())

		obs := observability.FromApp(app)
		obs.Channel("nonexistent").Info("should still appear")
	})

	if !strings.Contains(out, `"msg":"should still appear"`) {
		t.Fatalf("primary fallback did not emit record: %q", out)
	}
	if !strings.Contains(out, "unknown channel") {
		t.Fatalf("primary fallback should warn about unknown channel: %q", out)
	}
}

func TestChannel_OrderingError(t *testing.T) {
	_ = captureStdout(t, func() {
		// Channel registered BEFORE Module — should fail.
		app := framework.New("svc", "1.0.0").
			Use(observability.NewChannel("audit", observability.Config{
				Driver:      observability.DriverFile,
				LogFilePath: filepath.Join(t.TempDir(), "x.log"),
			})).
			Use(observability.Module)

		err := app.Init(context.Background())
		if err == nil {
			t.Fatalf("expected error when channel module precedes primary Module")
		}
		if !strings.Contains(err.Error(), "before Module") {
			t.Fatalf("err should hint at ordering: %v", err)
		}
	})
}

func TestChannel_PerChannelLevelFilter(t *testing.T) {
	// Audit channel restricted to warn+ via Config.LogLevel; info records
	// to that channel must be dropped while the primary still gets them.
	dir := t.TempDir()
	auditPath := filepath.Join(dir, "audit-level.log")

	_ = captureStdout(t, func() {
		t.Setenv("OBSERVABILITY_DRIVER", "stdout")

		app := framework.New("svc", "1.0.0").
			Use(observability.Module).
			Use(observability.NewChannel("audit", observability.Config{
				Driver:      observability.DriverFile,
				LogLevel:    slog.LevelWarn,
				LogFilePath: auditPath,
			}))

		if err := app.Init(context.Background()); err != nil {
			t.Fatalf("Init: %v", err)
		}
		defer app.Shutdown(context.Background())

		obs := observability.FromApp(app)
		obs.Channel("audit").Info("noisy-info-should-drop")
		obs.Channel("audit").Warn("important-warn-should-keep")
	})

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("ReadFile audit: %v", err)
	}
	if strings.Contains(string(data), "noisy-info-should-drop") {
		t.Errorf("per-channel warn filter broken — info record leaked: %q", string(data))
	}
	if !strings.Contains(string(data), "important-warn-should-keep") {
		t.Errorf("per-channel filter dropped warn: %q", string(data))
	}
}

func TestChannelsFromEnv_RegistersDeclaredChannels(t *testing.T) {
	dir := t.TempDir()
	auditPath := filepath.Join(dir, "env-audit.log")

	_ = captureStdout(t, func() {
		t.Setenv("OBSERVABILITY_DRIVER", "stdout")
		t.Setenv("OBSERVABILITY_CHANNELS", "audit,reports")

		// audit: file + warn+ filter (Laravel `daily` + level=warning)
		t.Setenv("OBSERVABILITY_CHANNEL_AUDIT_DRIVER", "file")
		t.Setenv("OBSERVABILITY_CHANNEL_AUDIT_LOG_LEVEL", "warn")
		t.Setenv("OBSERVABILITY_CHANNEL_AUDIT_LOG_FILE_PATH", auditPath)
		t.Setenv("OBSERVABILITY_CHANNEL_AUDIT_LOG_FILE_ROTATION", "none")

		// reports: stdout, debug
		t.Setenv("OBSERVABILITY_CHANNEL_REPORTS_DRIVER", "stdout")
		t.Setenv("OBSERVABILITY_CHANNEL_REPORTS_LOG_LEVEL", "debug")

		app := framework.New("svc", "1.0.0").
			Use(observability.Module).
			Use(observability.ChannelsFromEnv())

		if err := app.Init(context.Background()); err != nil {
			t.Fatalf("Init: %v", err)
		}
		defer app.Shutdown(context.Background())

		obs := observability.FromApp(app)
		names := obs.Channels()

		want := map[string]bool{"primary": false, "audit": false, "reports": false}
		for _, n := range names {
			want[n] = true
		}
		for k, ok := range want {
			if !ok {
				t.Errorf("missing channel %q in env-declared set %v", k, names)
			}
		}

		// Per-channel level: audit channel drops info, keeps warn.
		obs.Channel("audit").Info("dropped")
		obs.Channel("audit").Warn("kept")
	})

	data, err := os.ReadFile(auditPath)
	if err != nil {
		t.Fatalf("ReadFile audit: %v", err)
	}
	if strings.Contains(string(data), "dropped") {
		t.Errorf("env-declared audit channel did not honour LOG_LEVEL=warn: %q", string(data))
	}
	if !strings.Contains(string(data), "kept") {
		t.Errorf("env-declared audit channel missing warn record: %q", string(data))
	}
}

func TestChannelsFromEnv_NoopWhenUnset(t *testing.T) {
	_ = captureStdout(t, func() {
		t.Setenv("OBSERVABILITY_DRIVER", "stdout")
		t.Setenv("OBSERVABILITY_CHANNELS", "")

		app := framework.New("svc", "1.0.0").
			Use(observability.Module).
			Use(observability.ChannelsFromEnv())

		if err := app.Init(context.Background()); err != nil {
			t.Fatalf("Init: %v", err)
		}
		defer app.Shutdown(context.Background())

		names := observability.FromApp(app).Channels()
		if len(names) != 1 || names[0] != observability.PrimaryChannel {
			t.Errorf("expected only primary channel, got %v", names)
		}
	})
}

func TestChannelsFromEnv_RejectsPrimaryReserved(t *testing.T) {
	_ = captureStdout(t, func() {
		t.Setenv("OBSERVABILITY_DRIVER", "stdout")
		t.Setenv("OBSERVABILITY_CHANNELS", "primary,audit")

		app := framework.New("svc", "1.0.0").
			Use(observability.Module).
			Use(observability.ChannelsFromEnv())

		if err := app.Init(context.Background()); err == nil {
			t.Fatalf("expected error for reserved 'primary' in OBSERVABILITY_CHANNELS")
		} else if !strings.Contains(err.Error(), "reserved") {
			t.Fatalf("err should mention reserved: %v", err)
		}
	})
}

func TestChannelsFromEnv_RejectsDuplicate(t *testing.T) {
	_ = captureStdout(t, func() {
		t.Setenv("OBSERVABILITY_DRIVER", "stdout")
		t.Setenv("OBSERVABILITY_CHANNELS", "audit,audit")
		t.Setenv("OBSERVABILITY_CHANNEL_AUDIT_DRIVER", "stdout")

		app := framework.New("svc", "1.0.0").
			Use(observability.Module).
			Use(observability.ChannelsFromEnv())

		if err := app.Init(context.Background()); err == nil {
			t.Fatalf("expected error for duplicate channel name")
		} else if !strings.Contains(err.Error(), "twice") {
			t.Fatalf("err should mention duplicate: %v", err)
		}
	})
}

func TestChannelsFromEnv_OrderingErrorWhenBeforeModule(t *testing.T) {
	_ = captureStdout(t, func() {
		t.Setenv("OBSERVABILITY_DRIVER", "stdout")
		t.Setenv("OBSERVABILITY_CHANNELS", "audit")
		t.Setenv("OBSERVABILITY_CHANNEL_AUDIT_DRIVER", "stdout")

		// ChannelsFromEnv before Module — must fail.
		app := framework.New("svc", "1.0.0").
			Use(observability.ChannelsFromEnv()).
			Use(observability.Module)

		err := app.Init(context.Background())
		if err == nil {
			t.Fatalf("expected ordering error")
		}
		if !strings.Contains(err.Error(), "after observability.Module") {
			t.Fatalf("err should hint at ordering: %v", err)
		}
	})
}

func TestLoadChannelConfigFromEnv_NormalisesName(t *testing.T) {
	t.Setenv("OBSERVABILITY_CHANNEL_AUDIT_TRAIL_DRIVER", "file")
	t.Setenv("OBSERVABILITY_CHANNEL_AUDIT_TRAIL_LOG_LEVEL", "warn")

	cfg := observability.LoadChannelConfigFromEnv("audit-trail") // hyphen normalises to underscore
	if cfg.Driver != "file" {
		t.Errorf("Driver = %q, want file", cfg.Driver)
	}
	if cfg.LogLevel != slog.LevelWarn {
		t.Errorf("LogLevel = %v, want warn", cfg.LogLevel)
	}
}

func TestProvider_ChannelsLists(t *testing.T) {
	_ = captureStdout(t, func() {
		t.Setenv("OBSERVABILITY_DRIVER", "stdout")

		app := framework.New("svc", "1.0.0").
			Use(observability.Module).
			Use(observability.NewChannel("audit", observability.Config{Driver: observability.DriverStdout})).
			Use(observability.NewChannel("billing", observability.Config{Driver: observability.DriverStdout}))
		if err := app.Init(context.Background()); err != nil {
			t.Fatalf("Init: %v", err)
		}
		defer app.Shutdown(context.Background())

		names := observability.FromApp(app).Channels()
		want := map[string]bool{"primary": false, "audit": false, "billing": false}
		for _, n := range names {
			want[n] = true
		}
		for k, ok := range want {
			if !ok {
				t.Errorf("missing channel %q in %v", k, names)
			}
		}
	})
}
