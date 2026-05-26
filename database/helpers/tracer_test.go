package helpers

import "testing"

func TestExtractSQLCName(t *testing.T) {
	sql := `-- name: GetUser :one
SELECT id FROM users WHERE id = $1`
	if got := ExtractSQLCName(sql); got != "GetUser" {
		t.Fatalf("got %q", got)
	}
}

func TestSQLOperation(t *testing.T) {
	if got := SQLOperation("select * from users"); got != "SELECT" {
		t.Fatalf("got %q", got)
	}
}
