package auth

import (
	"testing"
)

func TestBeforeHookAllowBypassesPolicy(t *testing.T) {
	resetAuthTestState(t)
	ability := "update-" + t.Name()
	type Post struct{ AuthorID string }
	_ = RegisterPolicyFunc[Post](ability, func(p *Principal, post Post) bool {
		return false
	})
	allow := true
	Before(func(_ *Principal, _ string, _ ...any) *bool {
		return &allow
	})
	if !Authorize(ability, &Principal{SubjectID: "x"}, Post{AuthorID: "x"}) {
		t.Fatal("before allow should bypass deny policy")
	}
}

func TestAfterHookDeniesAllowedGate(t *testing.T) {
	resetAuthTestState(t)
	name := "gate-after-" + t.Name()
	MustDefine(name, func(p *Principal) bool { return p != nil })
	deny := false
	After(func(_ *Principal, _ string, _ ...any) *bool {
		return &deny
	})
	if Check(name, &Principal{SubjectID: "1"}) {
		t.Fatal("after deny should block allowed gate")
	}
}

func TestPolicyResourceAuthorization(t *testing.T) {
	resetAuthTestState(t)
	type Post struct{ AuthorID string }
	ability := "update-" + t.Name()
	_ = RegisterPolicyFunc[Post](ability, func(p *Principal, post Post) bool {
		return p != nil && post.AuthorID == p.SubjectID
	})
	if !Authorize(ability, &Principal{SubjectID: "alice"}, Post{AuthorID: "alice"}) {
		t.Fatal("owner should pass")
	}
	if Authorize(ability, &Principal{SubjectID: "bob"}, Post{AuthorID: "alice"}) {
		t.Fatal("non-owner should fail")
	}
}
