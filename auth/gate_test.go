package auth

import (
	"strings"
	"testing"
)

func TestDefineDuplicateReturnsError(t *testing.T) {
	name := "dup-gate-" + t.Name()
	if err := Define(name, func(*Principal) bool { return true }); err != nil {
		t.Fatal(err)
	}
	if err := Define(name, func(*Principal) bool { return false }); err == nil {
		t.Fatal("expected duplicate error")
	}
}

func TestMustDefinePanicsOnDuplicate(t *testing.T) {
	name := "must-dup-" + t.Name()
	MustDefine(name, func(*Principal) bool { return true })
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		} else if !strings.Contains(r.(error).Error(), "already defined") {
			t.Fatalf("panic=%v", r)
		}
	}()
	MustDefine(name, func(*Principal) bool { return false })
}

func TestCheckUnknownGateReturnsFalse(t *testing.T) {
	if Check("no-such-gate-"+t.Name(), &Principal{SubjectID: "x"}) {
		t.Fatal("expected false for undefined gate")
	}
}

func TestCheckNilPrincipal(t *testing.T) {
	name := "nil-principal-" + t.Name()
	MustDefine(name, func(p *Principal) bool { return p != nil })
	if Check(name, nil) {
		t.Fatal("expected false for nil principal")
	}
}
