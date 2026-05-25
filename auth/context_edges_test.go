package auth

import (
	"context"
	"testing"
)

func TestContextHelpersNilAndEmpty(t *testing.T) {
	if _, ok := PrincipalFromContext(nil); ok {
		t.Fatal("nil context should not have principal")
	}
	if _, ok := SubjectIDFromContext(context.Background()); ok {
		t.Fatal("empty context should not have subject")
	}
	if _, ok := UserIDFromContext(context.Background()); ok {
		t.Fatal("empty context should not have user id")
	}

	ctx := ContextWithPrincipal(context.Background(), &Principal{SubjectID: ""})
	if _, ok := SubjectIDFromContext(ctx); ok {
		t.Fatal("empty subject id should not be returned")
	}

	if _, ok := FromContext(nil); ok {
		t.Fatal("nil context should not have manager")
	}
}
