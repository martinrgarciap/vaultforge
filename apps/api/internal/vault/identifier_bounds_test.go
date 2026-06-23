package vault

import (
	"strings"
	"testing"
)

func TestValidIdentifierEnforcesByteBound(
	t *testing.T,
) {
	t.Parallel()

	testCases := []struct {
		name  string
		value string
		valid bool
	}{
		{
			name:  "ordinary identifier",
			value: "vault-request-123",
			valid: true,
		},
		{
			name: "maximum length",
			value: strings.Repeat(
				"a",
				maxIdentifierBytes,
			),
			valid: true,
		},
		{
			name: "above maximum length",
			value: strings.Repeat(
				"a",
				maxIdentifierBytes+1,
			),
			valid: false,
		},
		{
			name:  "empty",
			value: "",
			valid: false,
		},
		{
			name:  "contains line break",
			value: "vault\nidentifier",
			valid: false,
		},
	}

	for _, testCase := range testCases {
		testCase := testCase

		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if got := validIdentifier(
				testCase.value,
			); got != testCase.valid {
				t.Fatalf(
					"validIdentifier() = %t, want %t",
					got,
					testCase.valid,
				)
			}
		})
	}
}
