package httpclient

import (
	"context"
	"testing"

	"github.com/godx-jp/godx-platform-framework/framework"
	hdriver "github.com/godx-jp/godx-platform-framework/httpclient/driver"
)

func TestModuleWires(t *testing.T) {
	cfg := Config{
		Default: hdriver.DriverStdlib,
		Clients: map[string]ClientConfig{
			hdriver.DriverStdlib: {Driver: hdriver.DriverStdlib, Spec: hdriver.Spec{Name: hdriver.DriverStdlib}},
		},
	}
	app := framework.New("svc", "0.0.0").Use(ModuleWithConfig(cfg))
	if err := app.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	defer app.Shutdown(context.Background())
	mgr, err := FromApp(app)
	if err != nil || mgr.Default() == nil {
		t.Fatalf("FromApp: %v", err)
	}
}
