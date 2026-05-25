// Run: go run ./examples/httpclient from repo root.
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httptest"

	"github.com/godx-jp/godx-platform-framework/framework"
	"github.com/godx-jp/godx-platform-framework/httpclient"
	hdriver "github.com/godx-jp/godx-platform-framework/httpclient/driver"
)

func main() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"status":"ok"}`)
	}))
	defer srv.Close()

	ctx := context.Background()
	cfg := httpclient.Config{
		Default: hdriver.DriverStdlib,
		Clients: map[string]httpclient.ClientConfig{
			hdriver.DriverStdlib: {
				Driver: hdriver.DriverStdlib,
				Spec:   hdriver.Spec{Name: hdriver.DriverStdlib, BaseURL: srv.URL},
			},
		},
	}
	app := framework.New("httpclient-example", "0.0.0").Use(httpclient.ModuleWithConfig(cfg))
	if err := app.Init(ctx); err != nil {
		log.Fatalf("init: %v", err)
	}
	defer app.Shutdown(ctx)

	mgr, _ := httpclient.FromApp(app)
	resp, err := mgr.Get(ctx, "/api/status")
	if err != nil {
		log.Fatalf("Get: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("status=%d body=%s\n", resp.StatusCode, body)
}
