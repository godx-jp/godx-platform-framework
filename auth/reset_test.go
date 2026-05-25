package auth

import "testing"

func resetAuthTestState(t *testing.T) {
	t.Helper()
	ResetHooks()
	ResetPolicies()
}
