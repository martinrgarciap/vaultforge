package itemhandler

import "testing"

func FuzzDecodeItemCursor(f *testing.F) {
	f.Add("")
	f.Add("invalid")
	f.Add(
		encodeItemCursorTestJSON(
			`{"version":1,"updatedAt":"2026-06-23T16:30:45Z","id":"00000000-0000-0000-0000-000000002001"}`,
		),
	)

	f.Fuzz(func(t *testing.T, value string) {
		cursor, err := decodeItemCursor(value)
		if err != nil {
			return
		}

		if !cursor.Valid() {
			t.Fatal("accepted cursor was not valid")
		}

		encoded, err := encodeItemCursor(cursor)
		if err != nil {
			t.Fatal("accepted cursor could not be encoded")
		}

		roundTrip, err := decodeItemCursor(encoded)
		if err != nil {
			t.Fatal("encoded cursor could not be decoded")
		}

		if roundTrip.ID != cursor.ID || !roundTrip.UpdatedAt.Equal(cursor.UpdatedAt) {
			t.Fatal("cursor round trip changed its value")
		}
	})
}
