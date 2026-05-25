package featureflag_test

import (
	"context"
	"testing"

	fdriver "github.com/godx-jp/godx-platform-framework/featureflag/driver"
	_ "github.com/godx-jp/godx-platform-framework/featureflag/drivers/flagsmith"
	_ "github.com/godx-jp/godx-platform-framework/featureflag/drivers/launchdarkly"
	_ "github.com/godx-jp/godx-platform-framework/featureflag/drivers/openfeature"
	_ "github.com/godx-jp/godx-platform-framework/featureflag/drivers/unleash"
)

func TestHeavyDriversRegistered(t *testing.T) {
	for _, name := range []string{
		fdriver.DriverOpenFeature,
		fdriver.DriverLaunchDarkly,
		fdriver.DriverUnleash,
		fdriver.DriverFlagsmith,
	} {
		if fdriver.Lookup(name) == nil {
			t.Fatalf("driver %q not registered", name)
		}
	}
}

func TestHeavyStubsValidateConfig(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		spec fdriver.Spec
	}{
		{fdriver.DriverOpenFeature, fdriver.Spec{Name: fdriver.DriverOpenFeature}},
		{fdriver.DriverLaunchDarkly, fdriver.Spec{Name: fdriver.DriverLaunchDarkly}},
		{fdriver.DriverUnleash, fdriver.Spec{Name: fdriver.DriverUnleash}},
		{fdriver.DriverFlagsmith, fdriver.Spec{Name: fdriver.DriverFlagsmith}},
	} {
		c := fdriver.Lookup(tc.name)
		if _, err := c(ctx, tc.spec); err == nil {
			t.Fatalf("%s should require config", tc.name)
		}
	}
}

func TestHeavyStubReturnsNotConfigured(t *testing.T) {
	ctx := context.Background()
	c := fdriver.Lookup(fdriver.DriverLaunchDarkly)
	p, err := c(ctx, fdriver.Spec{Name: fdriver.DriverLaunchDarkly, SDKKey: "sdk-test"})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = p.Enabled(ctx, "flag", "user", nil)
	if err != fdriver.ErrNotConfigured {
		t.Fatalf("err=%v want ErrNotConfigured", err)
	}
}
