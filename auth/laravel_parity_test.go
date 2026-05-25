package auth

import (
	"testing"
)

// Laravel Gate parity: AuthAccessGateTest patterns mapped to our GateFunc API.
// Reference: laravel/framework tests/Auth/AuthAccessGateTest.php

func TestLaravelGateDefineAllowAndDeny(t *testing.T) {
	allow := "laravel-allow-" + t.Name()
	deny := "laravel-deny-" + t.Name()
	admin := &Principal{SubjectID: "1", Roles: []string{"admin"}}

	if err := Define(allow, func(p *Principal) bool { return p != nil && HasRole(p, "admin") }); err != nil {
		t.Fatal(err)
	}
	if err := Define(deny, func(p *Principal) bool { return false }); err != nil {
		t.Fatal(err)
	}

	if !Check(allow, admin) {
		t.Fatal("expected allow gate to pass for admin")
	}
	if Check(deny, admin) {
		t.Fatal("expected deny gate to fail")
	}
}

func TestLaravelGateMissingAbilityReturnsFalse(t *testing.T) {
	// Laravel: $gate->check('missing') === false
	if Check("definitely-missing-gate-"+t.Name(), &Principal{SubjectID: "1"}) {
		t.Fatal("missing gate should deny")
	}
}

func TestLaravelGateNilPrincipalLikeGuest(t *testing.T) {
	name := "laravel-guest-gate-" + t.Name()
	MustDefine(name, func(p *Principal) bool {
		// Laravel closures with ?User allow guests when returning true for nil
		return p == nil
	})
	if !Check(name, nil) {
		t.Fatal("gate explicitly allowing nil principal should pass")
	}

	strict := "laravel-strict-gate-" + t.Name()
	MustDefine(strict, func(p *Principal) bool {
		return p != nil && p.SubjectID != ""
	})
	if Check(strict, nil) {
		t.Fatal("strict gate should deny nil principal")
	}
}

func TestLaravelGateDefineRejectsInvalidInput(t *testing.T) {
	if err := Define("", func(*Principal) bool { return true }); err == nil {
		t.Fatal("empty name should error")
	}
	if err := Define("valid-name", nil); err == nil {
		t.Fatal("nil fn should error")
	}
}

func TestLaravelHasRoleAnyOfArrayAbilities(t *testing.T) {
	// Laravel Gate::any / array abilities in allows()
	p := &Principal{Roles: []string{"editor"}}
	if !HasRole(p, "admin", "editor", "viewer") {
		t.Fatal("expected any-role match like Laravel ability middleware")
	}
	if HasRole(p, "admin", "viewer") {
		t.Fatal("expected no match")
	}
}

func TestLaravelHasPermissionAnyOfArrayAbilities(t *testing.T) {
	p := &Principal{Permissions: []string{"posts:read"}}
	if !HasPermission(p, "posts:write", "posts:read") {
		t.Fatal("expected any-permission match")
	}
}

func TestLaravelGateORComposition(t *testing.T) {
	// Laravel: return $user->isAdmin() || $user->can('edit-posts')
	name := "laravel-or-gate-" + t.Name()
	MustDefine(name, func(p *Principal) bool {
		return HasRole(p, "admin") || HasPermission(p, "posts:edit")
	})

	if !Check(name, &Principal{Roles: []string{"viewer"}, Permissions: []string{"posts:edit"}}) {
		t.Fatal("permission branch should allow")
	}
	if !Check(name, &Principal{Roles: []string{"admin"}}) {
		t.Fatal("role branch should allow")
	}
	if Check(name, &Principal{Roles: []string{"viewer"}}) {
		t.Fatal("expected deny")
	}
}
