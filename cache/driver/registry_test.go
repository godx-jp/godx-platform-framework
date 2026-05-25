package driver_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/godx-jp/godx-platform-framework/cache/driver"
	_ "github.com/godx-jp/godx-platform-framework/cache/drivers/memory" // re-registers under "memory"
)

func TestRegister_PanicsOnEmptyName(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on empty name")
		}
	}()
	driver.Register("", func(context.Context, driver.Spec) (driver.Driver, error) {
		return nil, nil
	})
}

func TestRegister_PanicsOnNilConstructor(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on nil constructor")
		}
	}()
	driver.Register("ghost", nil)
}

func TestRegister_LatestWins(t *testing.T) {
	calls := 0
	driver.Register("re-register-fixture", func(context.Context, driver.Spec) (driver.Driver, error) {
		calls++
		return nil, errors.New("v1")
	})
	driver.Register("re-register-fixture", func(context.Context, driver.Spec) (driver.Driver, error) {
		return nil, errors.New("v2")
	})
	_, err := driver.New(context.Background(), driver.Spec{Name: "re-register-fixture"})
	if err == nil || err.Error() != "v2" {
		t.Fatalf("latest Register should win; got err=%v", err)
	}
	if calls != 0 {
		t.Fatalf("first constructor was called %d times; want 0", calls)
	}
}

func TestLookup_ReturnsNilForMissing(t *testing.T) {
	if c := driver.Lookup("does-not-exist"); c != nil {
		t.Fatalf("Lookup returned %v for missing driver", c)
	}
}

func TestNames_IncludesAutoRegisteredMemory(t *testing.T) {
	names := driver.Names()
	found := false
	for _, n := range names {
		if n == driver.DriverMemory {
			found = true
		}
	}
	if !found {
		t.Fatalf("memory driver missing from Names(); have %v", names)
	}
	// Ensure sorted output for deterministic logging.
	prev := ""
	for _, n := range names {
		if n < prev {
			t.Fatalf("Names() not sorted: %v", names)
		}
		prev = n
	}
}

func TestNew_EmptyNameError(t *testing.T) {
	_, err := driver.New(context.Background(), driver.Spec{})
	if err == nil || !strings.Contains(err.Error(), "driver name is required") {
		t.Fatalf("want name-required error, got %v", err)
	}
}

func TestNew_MissingDriverHintsImportPath(t *testing.T) {
	_, err := driver.New(context.Background(), driver.Spec{Name: "ghost"})
	if err == nil {
		t.Fatal("want error for unknown driver")
	}
	msg := err.Error()
	if !strings.Contains(msg, "not registered") || !strings.Contains(msg, "drivers/ghost") {
		t.Fatalf("error should hint at the missing import; got: %v", msg)
	}
}

func TestSentinelsAreDistinct(t *testing.T) {
	// Defensive: every sentinel must be distinguishable via errors.Is.
	cases := []error{
		driver.ErrNotSupported,
		driver.ErrNotImplemented,
		driver.ErrNotInteger,
		driver.ErrClosed,
	}
	for i, a := range cases {
		for j, b := range cases {
			if i == j {
				continue
			}
			if errors.Is(a, b) {
				t.Fatalf("sentinels collide: %v matches %v via errors.Is", a, b)
			}
		}
	}
}
