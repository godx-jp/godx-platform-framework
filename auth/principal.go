package auth

import (
	"slices"

	adriver "github.com/godx-jp/godx-platform-framework/auth/driver"
)

// Principal is the authenticated identity for application code.
type Principal = adriver.Principal

// ActorKind classifies who is acting.
type ActorKind = adriver.ActorKind

const (
	ActorHuman      = adriver.ActorHuman
	ActorService    = adriver.ActorService
	ActorDevice     = adriver.ActorDevice
	ActorThirdParty = adriver.ActorThirdParty
)

// HasRole reports whether p has any of the given roles.
func HasRole(p *Principal, roles ...string) bool {
	if p == nil {
		return false
	}
	for _, role := range roles {
		if slices.Contains(p.Roles, role) {
			return true
		}
	}
	return false
}

// HasPermission reports whether p has any of the given permissions.
func HasPermission(p *Principal, perms ...string) bool {
	if p == nil {
		return false
	}
	for _, perm := range perms {
		if slices.Contains(p.Permissions, perm) {
			return true
		}
	}
	return false
}

// HasActorKind reports whether p matches any of the given actor kinds.
func HasActorKind(p *Principal, kinds ...ActorKind) bool {
	if p == nil {
		return false
	}
	for _, kind := range kinds {
		if p.ActorKind == kind {
			return true
		}
	}
	return false
}
