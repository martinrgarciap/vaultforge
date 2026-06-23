package itemhandler

import (
	"errors"
	"strings"
	"testing"
)

func TestParseItemETagRejectsOversizedValue(
	t *testing.T,
) {
	t.Parallel()

	value := `"` +
		strings.Repeat(
			"1",
			maxIfMatchHeaderBytes,
		) +
		`"`

	_, err := parseItemETag(value)

	if !errors.Is(err, errIfMatchInvalid) {
		t.Fatalf(
			"parseItemETag() error = %v, want %v",
			err,
			errIfMatchInvalid,
		)
	}
}
