package db

import (
	"strings"
	"testing"
)

func TestPostgresSpanNameDoesNotExposeStatement(t *testing.T) {
	const secretMarker = "synthetic-database-secret-marker"

	tests := []struct {
		statement string
		expected  string
	}{
		{
			statement: "SELECT password_hash FROM users WHERE email = '" + secretMarker + "'",
			expected:  "postgres.select",
		},
		{
			statement: "INSERT INTO vault_items VALUES ('" + secretMarker + "')",
			expected:  "postgres.insert",
		},
		{
			statement: "",
			expected:  "postgres.query",
		},
	}

	for _, test := range tests {
		name := postgresSpanName(test.statement)

		if name != test.expected {
			t.Fatalf("span name = %q, want %q", name, test.expected)
		}

		if strings.Contains(name, secretMarker) {
			t.Fatal("span name exposed SQL contents")
		}
	}
}
