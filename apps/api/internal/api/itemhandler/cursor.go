package itemhandler

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

const (
	itemCursorTokenVersion   = 1
	maxEncodedItemCursorSize = 1024
)

var errItemCursorInvalid = errors.New("item cursor is invalid")

type itemCursorToken struct {
	Version   int       `json:"version"`
	UpdatedAt time.Time `json:"updatedAt"`
	ID        string    `json:"id"`
}

func encodeItemCursor(cursor vault.ItemCursor) (string, error) {
	if !cursor.Valid() {
		return "", errItemCursorInvalid
	}

	encodedJSON, err := json.Marshal(
		itemCursorToken{
			Version:   itemCursorTokenVersion,
			UpdatedAt: cursor.UpdatedAt.UTC(),
			ID:        cursor.ID,
		},
	)
	if err != nil {
		return "", errItemCursorInvalid
	}

	return base64.RawURLEncoding.EncodeToString(encodedJSON), nil
}

func decodeItemCursor(value string) (vault.ItemCursor, error) {
	if value == "" ||
		value != strings.TrimSpace(value) ||
		len(value) > maxEncodedItemCursorSize {
		return vault.ItemCursor{}, errItemCursorInvalid
	}

	decodedJSON, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(decodedJSON) == 0 {
		return vault.ItemCursor{}, errItemCursorInvalid
	}

	decoder := json.NewDecoder(bytes.NewReader(decodedJSON))
	decoder.DisallowUnknownFields()

	var token itemCursorToken

	if err := decoder.Decode(&token); err != nil {
		return vault.ItemCursor{}, errItemCursorInvalid
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return vault.ItemCursor{}, errItemCursorInvalid
	}

	cursor := vault.ItemCursor{
		UpdatedAt: token.UpdatedAt.UTC(),
		ID:        token.ID,
	}

	if token.Version != itemCursorTokenVersion || !cursor.Valid() {
		return vault.ItemCursor{}, errItemCursorInvalid
	}

	return cursor, nil
}
