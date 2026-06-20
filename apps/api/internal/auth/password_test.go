package auth

import (
	"errors"
	"strings"
	"testing"
)

func TestValidatePassword(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		password string
		wantErr  error
	}{
		{
			name:     "accepts minimum length",
			password: strings.Repeat("a", MinPasswordLength),
		},
		{
			name:     "accepts maximum length",
			password: strings.Repeat("a", MaxPasswordLength),
		},
		{
			name:     "rejects empty password",
			password: "",
			wantErr:  ErrPasswordTooShort,
		},
		{
			name: "rejects password below minimum length",
			password: strings.Repeat(
				"a",
				MinPasswordLength-1,
			),
			wantErr: ErrPasswordTooShort,
		},
		{
			name: "rejects password above maximum length",
			password: strings.Repeat(
				"a",
				MaxPasswordLength+1,
			),
			wantErr: ErrPasswordTooLong,
		},
		{
			name: "counts unicode code points instead of bytes",
			password: strings.Repeat(
				"密",
				MinPasswordLength,
			),
		},
		{
			name: "accepts spaces without trimming",
			password: strings.Repeat(
				" ",
				MinPasswordLength,
			),
		},
		{
			name:     "rejects invalid UTF-8",
			password: string([]byte{0xff}),
			wantErr:  ErrPasswordInvalidUTF8,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := ValidatePassword(test.password)

			if test.wantErr == nil {
				if err != nil {
					t.Fatalf(
						"ValidatePassword() returned unexpected error: %v",
						err,
					)
				}

				return
			}

			if !errors.Is(err, test.wantErr) {
				t.Fatalf(
					"ValidatePassword() error = %v, want %v",
					err,
					test.wantErr,
				)
			}
		})
	}
}

func TestNormalizeAndValidatePassword(t *testing.T) {
	t.Parallel()

	t.Run(
		"normalizes canonically equivalent unicode to NFC",
		func(t *testing.T) {
			t.Parallel()

			decomposedPassword := strings.Repeat(
				"e\u0301",
				MinPasswordLength,
			)
			wantPassword := strings.Repeat(
				"é",
				MinPasswordLength,
			)

			gotPassword, err :=
				NormalizeAndValidatePassword(
					decomposedPassword,
				)
			if err != nil {
				t.Fatalf(
					"NormalizeAndValidatePassword() returned unexpected error: %v",
					err,
				)
			}

			if gotPassword != wantPassword {
				t.Fatalf(
					"NormalizeAndValidatePassword() = %q, want %q",
					gotPassword,
					wantPassword,
				)
			}
		},
	)

	t.Run(
		"preserves leading and trailing spaces",
		func(t *testing.T) {
			t.Parallel()

			password := "  correct horse battery staple  "

			gotPassword, err :=
				NormalizeAndValidatePassword(password)
			if err != nil {
				t.Fatalf(
					"NormalizeAndValidatePassword() returned unexpected error: %v",
					err,
				)
			}

			if gotPassword != password {
				t.Fatalf(
					"NormalizeAndValidatePassword() = %q, want %q",
					gotPassword,
					password,
				)
			}
		},
	)
}
