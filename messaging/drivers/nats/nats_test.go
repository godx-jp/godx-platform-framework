package nats_test

import (
	"context"
	"strings"
	"testing"

	mdriver "github.com/godx-jp/godx-platform-framework/messaging/driver"
	natsdrv "github.com/godx-jp/godx-platform-framework/messaging/drivers/nats"
)

func TestRegisteredOnImport(t *testing.T) {
	if mdriver.Lookup(mdriver.DriverNATS) == nil {
		t.Fatal("nats driver not registered")
	}
}

func TestNewRequiresURL(t *testing.T) {
	_, err := natsdrv.New(context.Background(), mdriver.Spec{Name: mdriver.DriverNATS})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "NATSURL") {
		t.Fatalf("error=%v", err)
	}
}
