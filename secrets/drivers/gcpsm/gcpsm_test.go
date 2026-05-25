package gcpsm

import (
	"context"
	"testing"

	sdriver "github.com/godx-jp/godx-platform-framework/secrets/driver"
)

func TestRegisteredOnImport(t *testing.T) {
	if sdriver.Lookup(sdriver.DriverGCPSM) == nil {
		t.Fatalf("gcpsm driver not registered")
	}
}

func TestConstructorValidatesProject(t *testing.T) {
	c := sdriver.Lookup(sdriver.DriverGCPSM)
	if _, err := c(context.Background(), sdriver.Spec{Name: sdriver.DriverGCPSM}); err == nil {
		t.Fatalf("expected error for missing project")
	}
}

func TestSecretIDNormalisation(t *testing.T) {
	s := &store{project: "p", prefix: "myapp"}
	for _, tc := range []struct{ in, want string }{
		{"db/password", "myapp-db-password"},
		{"db.password", "myapp-db-password"},
		{"some key", "myapp-some-key"},
		{"plain", "myapp-plain"},
	} {
		if got := s.secretID(tc.in); got != tc.want {
			t.Fatalf("secretID(%q)=%q want %q", tc.in, got, tc.want)
		}
	}

	s2 := &store{project: "p"}
	if got := s2.secretID("db/password"); got != "db-password" {
		t.Fatalf("no-prefix secretID=%q", got)
	}
}

func TestPathHelpers(t *testing.T) {
	s := &store{project: "p", prefix: "app"}
	if got := s.parentPath(); got != "projects/p" {
		t.Fatalf("parent=%q", got)
	}
	if got := s.secretPath("db"); got != "projects/p/secrets/app-db" {
		t.Fatalf("secret=%q", got)
	}
	if got := s.versionPath("db"); got != "projects/p/secrets/app-db/versions/latest" {
		t.Fatalf("version=%q", got)
	}
}

func TestNotInitialisedShutdownIdempotent(t *testing.T) {
	s := &store{project: "p"}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("first: %v", err)
	}
	if err := s.Shutdown(context.Background()); err != nil {
		t.Fatalf("second: %v", err)
	}
	if _, err := s.Get(context.Background(), "k"); err != sdriver.ErrClosed {
		t.Fatalf("Get err=%v", err)
	}
}
