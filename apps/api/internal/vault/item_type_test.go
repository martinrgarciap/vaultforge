package vault

import (
	"errors"
	"testing"
)

func TestParseItemTypeAcceptsSupportedTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  ItemType
	}{
		{
			name:  "login",
			value: "login",
			want:  ItemTypeLogin,
		},
		{
			name:  "API key",
			value: "api_key",
			want:  ItemTypeAPIKey,
		},
		{
			name:  "environment variable",
			value: "environment_variable",
			want:  ItemTypeEnvironmentVariable,
		},
		{
			name:  "database connection",
			value: "database_connection",
			want:  ItemTypeDatabaseConnection,
		},
		{
			name:  "secure note",
			value: "secure_note",
			want:  ItemTypeSecureNote,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseItemType(test.value)
			if err != nil {
				t.Fatalf("ParseItemType() error = %v", err)
			}

			if got != test.want {
				t.Fatalf("ParseItemType() = %q, want %q", got, test.want)
			}

			if !got.Valid() {
				t.Fatalf("parsed item type %q was not valid", got)
			}
		})
	}
}

func TestParseItemTypeRejectsUnsupportedTypes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
	}{
		{
			name:  "empty",
			value: "",
		},
		{
			name:  "unknown type",
			value: "credit_card",
		},
		{
			name:  "uppercase",
			value: "LOGIN",
		},
		{
			name:  "leading whitespace",
			value: " login",
		},
		{
			name:  "trailing whitespace",
			value: "login ",
		},
		{
			name:  "hyphenated",
			value: "api-key",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseItemType(test.value)

			if !errors.Is(err, ErrItemTypeInvalid) {
				t.Fatalf("ParseItemType() error = %v, want %v", err, ErrItemTypeInvalid)
			}

			if got != "" {
				t.Fatalf("ParseItemType() = %q, want empty item type", got)
			}
		})
	}
}

func TestItemTypeValidRejectsZeroAndUnknownValues(t *testing.T) {
	t.Parallel()

	tests := []ItemType{
		"",
		"unknown",
		"login_record",
	}

	for _, itemType := range tests {
		if itemType.Valid() {
			t.Fatalf("ItemType(%q).Valid() = true, want false", itemType)
		}
	}
}
