package auth

import "testing"

func TestHasRole(t *testing.T) {
	p := &Principal{Roles: []string{"admin", "editor"}}
	if !HasRole(p, "admin") || !HasRole(p, "editor") {
		t.Fatal("expected role match")
	}
	if HasRole(p, "viewer") || HasRole(nil, "admin") {
		t.Fatal("expected no match")
	}
}

func TestHasPermission(t *testing.T) {
	p := &Principal{Permissions: []string{"posts:read", "posts:write"}}
	if !HasPermission(p, "posts:read") {
		t.Fatal("expected permission match")
	}
	if HasPermission(p, "posts:delete") || HasPermission(nil, "x") {
		t.Fatal("expected no match")
	}
}

func TestHasActorKind(t *testing.T) {
	p := &Principal{ActorKind: ActorService}
	if !HasActorKind(p, ActorService) {
		t.Fatal("expected kind match")
	}
	if HasActorKind(p, ActorHuman) || HasActorKind(nil, ActorHuman) {
		t.Fatal("expected no match")
	}
}
