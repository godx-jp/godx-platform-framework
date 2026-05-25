package observability_test

import (
	"context"
	"encoding/json"
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
