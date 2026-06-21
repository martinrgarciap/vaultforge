package vault

import (
	"errors"
	"testing"
)

func TestValidateExpectedItemVersionAcceptsPositiveVersions(t *testing.T) {
	t.Parallel()

	for _, version := range []int{1, 2, 100} {
		if err := ValidateExpectedItemVersion(version); err != nil {
			t.Fatalf(
				"ValidateExpectedItemVersion(%d) error = %v",
				version,
				err,
			)
		}
	}
}

func TestValidateExpectedItemVersionRejectsNonPositiveVersions(t *testing.T) {
	t.Parallel()

	for _, version := range []int{-1, 0} {
		err := ValidateExpectedItemVersion(version)

		if !errors.Is(err, ErrItemVersionInvalid) {
			t.Fatalf(
				"ValidateExpectedItemVersion(%d) error = %v, want %v",
				version,
				err,
				ErrItemVersionInvalid,
			)
		}
	}
}
