package vault

import (
	"errors"
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

func TestNormalizeNameTrimsAndNormalizesUnicode(
	t *testing.T,
) {
	t.Parallel()

	input := "  De\u0301velopment Vault  "

	normalizedName, err := NormalizeName(input)
	if err != nil {
		t.Fatalf(
			"normalize vault name: %v",
			err,
		)
	}

	expectedName := "Dévelopment Vault"

	if normalizedName != expectedName {
		t.Fatalf(
			"normalized name = %q, want %q",
			normalizedName,
			expectedName,
		)
	}

	if !norm.NFC.IsNormalString(
		normalizedName,
	) {
		t.Fatal(
			"vault name was not normalized to NFC",
		)
	}
}

func TestNormalizeNameAcceptsMaximumLength(
	t *testing.T,
) {
	t.Parallel()

	input := strings.Repeat(
		"é",
		MaxVaultNameLength,
	)

	normalizedName, err := NormalizeName(input)
	if err != nil {
		t.Fatalf(
			"normalize maximum-length name: %v",
			err,
		)
	}

	if utf8.RuneCountInString(
		normalizedName,
	) != MaxVaultNameLength {
		t.Fatalf(
			"normalized length = %d, want %d",
			utf8.RuneCountInString(
				normalizedName,
			),
			MaxVaultNameLength,
		)
	}
}

func TestNormalizeNameRejectsInvalidValues(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantError error
	}{
		{
			name:      "empty",
			input:     "",
			wantError: ErrVaultNameEmpty,
		},
		{
			name:      "whitespace only",
			input:     "   ",
			wantError: ErrVaultNameEmpty,
		},
		{
			name: "too long",
			input: strings.Repeat(
				"a",
				MaxVaultNameLength+1,
			),
			wantError: ErrVaultNameTooLong,
		},
		{
			name:      "line break",
			input:     "Development\nVault",
			wantError: ErrVaultNameContainsControlCharacter,
		},
		{
			name:      "tab",
			input:     "Development\tVault",
			wantError: ErrVaultNameContainsControlCharacter,
		},
		{
			name: "invalid UTF-8",
			input: string(
				[]byte{0xff, 0xfe},
			),
			wantError: ErrVaultNameInvalidUTF8,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := NormalizeName(
				test.input,
			)

			if !errors.Is(
				err,
				test.wantError,
			) {
				t.Fatalf(
					"NormalizeName() error = %v, want %v",
					err,
					test.wantError,
				)
			}
		})
	}
}
