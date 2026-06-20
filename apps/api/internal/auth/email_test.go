package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestNormalizeEmail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		email     string
		wantEmail string
		wantErr   error
	}{
		{
			name:      "accepts valid email",
			email:     "martin@example.com",
			wantEmail: "martin@example.com",
		},
		{
			name:      "trims and lowercases email",
			email:     "  Martin.Garcia@Example.COM  ",
			wantEmail: "martin.garcia@example.com",
		},
		{
			name:      "preserves dots and plus alias",
			email:     "Martin.Test+Vault@Example.COM",
			wantEmail: "martin.test+vault@example.com",
		},
		{
			name:    "rejects empty email",
			email:   "",
			wantErr: ErrEmailInvalid,
		},
		{
			name:    "rejects whitespace email",
			email:   "   ",
			wantErr: ErrEmailInvalid,
		},
		{
			name:    "rejects missing at sign",
			email:   "martin.example.com",
			wantErr: ErrEmailInvalid,
		},
		{
			name:    "rejects display name",
			email:   "Martin <martin@example.com>",
			wantErr: ErrEmailInvalid,
		},
		{
			name:    "rejects multiple addresses",
			email:   "one@example.com, two@example.com",
			wantErr: ErrEmailInvalid,
		},
		{
			name:    "rejects newline",
			email:   "martin@example.com\ninvalid",
			wantErr: ErrEmailInvalid,
		},
		{
			name: "rejects oversized email",
			email: strings.Repeat(
				"a",
				MaxEmailLength,
			) + "@example.com",
			wantErr: ErrEmailInvalid,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			gotEmail, err := NormalizeEmail(
				test.email,
			)

			if test.wantErr == nil {
				if err != nil {
					t.Fatalf(
						"NormalizeEmail() returned unexpected error: %v",
						err,
					)
				}

				if gotEmail != test.wantEmail {
					t.Fatalf(
						"NormalizeEmail() = %q, want %q",
						gotEmail,
						test.wantEmail,
					)
				}

				return
			}

			if !errors.Is(err, test.wantErr) {
				t.Fatalf(
					"NormalizeEmail() error = %v, want %v",
					err,
					test.wantErr,
				)
			}
		})
	}
}
