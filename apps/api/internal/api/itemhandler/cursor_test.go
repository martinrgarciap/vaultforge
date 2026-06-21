package itemhandler

import (
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

const itemHandlerTestCursorID = "00000000-0000-0000-0000-000000002001"

func TestItemCursorRoundTrip(t *testing.T) {
	t.Parallel()

	location := time.FixedZone("cursor-test", -4*60*60)
	updatedAt := time.Date(
		2026,
		time.June,
		23,
		12,
		30,
		45,
		123456789,
		location,
	)

	encoded, err := encodeItemCursor(
		vault.ItemCursor{
			UpdatedAt: updatedAt,
			ID:        itemHandlerTestCursorID,
		},
	)
	if err != nil {
		t.Fatalf("encodeItemCursor() error = %v", err)
	}

	if encoded == "" {
		t.Fatal("encoded item cursor was empty")
	}

	decoded, err := decodeItemCursor(encoded)
	if err != nil {
		t.Fatalf("decodeItemCursor() error = %v", err)
	}

	if decoded.ID != itemHandlerTestCursorID {
		t.Fatalf(
			"decoded cursor ID = %q, want %q",
			decoded.ID,
			itemHandlerTestCursorID,
		)
	}

	if !decoded.UpdatedAt.Equal(updatedAt) {
		t.Fatalf(
			"decoded cursor time = %v, want %v",
			decoded.UpdatedAt,
			updatedAt,
		)
	}

	if decoded.UpdatedAt.Location() != time.UTC {
		t.Fatalf(
			"decoded cursor location = %v, want UTC",
			decoded.UpdatedAt.Location(),
		)
	}
}

func TestEncodeItemCursorRejectsInvalidCursor(t *testing.T) {
	t.Parallel()

	tests := []vault.ItemCursor{
		{
			ID: itemHandlerTestCursorID,
		},
		{
			UpdatedAt: time.Now().UTC(),
		},
		{
			UpdatedAt: time.Now().UTC(),
			ID:        " " + itemHandlerTestCursorID,
		},
	}

	for _, cursor := range tests {
		_, err := encodeItemCursor(cursor)

		if !errors.Is(err, errItemCursorInvalid) {
			t.Fatalf(
				"encodeItemCursor(%+v) error = %v, want %v",
				cursor,
				err,
				errItemCursorInvalid,
			)
		}
	}
}

func TestDecodeItemCursorRejectsInvalidTokens(t *testing.T) {
	t.Parallel()

	updatedAt := "2026-06-23T16:30:45.123456789Z"

	tests := []struct {
		name  string
		value string
	}{
		{
			name:  "empty",
			value: "",
		},
		{
			name:  "surrounding whitespace",
			value: " invalid-token ",
		},
		{
			name:  "invalid base64",
			value: "%%%invalid%%%",
		},
		{
			name: "unsupported version",
			value: encodeItemCursorTestJSON(
				`{"version":2,"updatedAt":"` +
					updatedAt +
					`","id":"` +
					itemHandlerTestCursorID +
					`"}`,
			),
		},
		{
			name: "zero timestamp",
			value: encodeItemCursorTestJSON(
				`{"version":1,"updatedAt":"0001-01-01T00:00:00Z","id":"` +
					itemHandlerTestCursorID +
					`"}`,
			),
		},
		{
			name: "blank ID",
			value: encodeItemCursorTestJSON(
				`{"version":1,"updatedAt":"` +
					updatedAt +
					`","id":" "}`,
			),
		},
		{
			name: "unknown field",
			value: encodeItemCursorTestJSON(
				`{"version":1,"updatedAt":"` +
					updatedAt +
					`","id":"` +
					itemHandlerTestCursorID +
					`","unexpected":true}`,
			),
		},
		{
			name: "multiple JSON values",
			value: encodeItemCursorTestJSON(
				`{"version":1,"updatedAt":"` +
					updatedAt +
					`","id":"` +
					itemHandlerTestCursorID +
					`"} {}`,
			),
		},
		{
			name: "oversized",
			value: string(
				make(
					[]byte,
					maxEncodedItemCursorSize+1,
				),
			),
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := decodeItemCursor(test.value)

			if !errors.Is(err, errItemCursorInvalid) {
				t.Fatalf(
					"decodeItemCursor() error = %v, want %v",
					err,
					errItemCursorInvalid,
				)
			}
		})
	}
}

func encodeItemCursorTestJSON(value string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}
