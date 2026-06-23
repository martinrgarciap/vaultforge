package itemhandler

import (
	"strings"
	"testing"
)

func FuzzParseItemETag(f *testing.F) {
	f.Add(`"1"`)
	f.Add(` "27" `)
	f.Add(`W/"1"`)
	f.Add("")

	f.Fuzz(func(t *testing.T, value string) {
		version, err := parseItemETag(value)
		if err != nil {
			return
		}

		if version < 1 {
			t.Fatal("accepted item version was not positive")
		}

		if itemETag(version) != strings.TrimSpace(value) {
			t.Fatal("accepted item ETag was not canonical")
		}
	})
}
