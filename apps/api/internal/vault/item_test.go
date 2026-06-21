package vault

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestNormalizeSyntheticItemPayloadCanonicalizesObject(t *testing.T) {
	t.Parallel()

	payload := json.RawMessage(`{
	"username": "synthetic-user",
	"metadata": {
		"z": 2,
		"a": 1
	},
	"enabled": true
}`)

	normalized, err := NormalizeSyntheticItemPayload(payload)
	if err != nil {
		t.Fatalf("NormalizeSyntheticItemPayload() error = %v", err)
	}

	const want = `{"enabled":true,"metadata":{"a":1,"z":2},"username":"synthetic-user"}`

	if string(normalized) != want {
		t.Fatalf("normalized payload = %s, want %s", normalized, want)
	}

}

func TestNormalizeSyntheticItemPayloadAcceptsEmptyObject(t *testing.T) {
	t.Parallel()

	normalized, err := NormalizeSyntheticItemPayload(json.RawMessage(` { } `))
	if err != nil {
		t.Fatalf("NormalizeSyntheticItemPayload() error = %v", err)
	}

	if string(normalized) != `{}` {
		t.Fatalf("normalized payload = %s, want {}", normalized)
	}

}

func TestNormalizeSyntheticItemPayloadRejectsInvalidPayloads(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		payload json.RawMessage
		wantErr error
	}{
		{
			name:    "nil payload",
			payload: nil,
			wantErr: ErrItemPayloadEmpty,
		},
		{
			name:    "blank payload",
			payload: json.RawMessage("   \n\t"),
			wantErr: ErrItemPayloadEmpty,
		},
		{
			name:    "malformed JSON",
			payload: json.RawMessage(`{"username":`),
			wantErr: ErrItemPayloadInvalid,
		},
		{
			name:    "multiple JSON values",
			payload: json.RawMessage(`{"username":"one"} {"username":"two"}`),
			wantErr: ErrItemPayloadInvalid,
		},
		{
			name:    "array",
			payload: json.RawMessage(`["synthetic-value"]`),
			wantErr: ErrItemPayloadNotObject,
		},
		{
			name:    "string",
			payload: json.RawMessage(`"synthetic-value"`),
			wantErr: ErrItemPayloadNotObject,
		},
		{
			name:    "null",
			payload: json.RawMessage(`null`),
			wantErr: ErrItemPayloadNotObject,
		},
		{
			name:    "invalid UTF-8",
			payload: json.RawMessage([]byte{'{', '"', 'x', '"', ':', '"', 0xff, '"', '}'}),
			wantErr: ErrItemPayloadInvalid,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			normalized, err := NormalizeSyntheticItemPayload(test.payload)

			if !errors.Is(err, test.wantErr) {
				t.Fatalf(
					"NormalizeSyntheticItemPayload() error = %v, want %v",
					err,
					test.wantErr,
				)
			}

			if normalized != nil {
				t.Fatalf("normalized payload = %s, want nil", normalized)
			}
		})
	}

}

func TestNormalizeSyntheticItemPayloadRejectsOversizedPayload(t *testing.T) {
	t.Parallel()

	payload := json.RawMessage(
		`{"value":"` +
			strings.Repeat("a", MaxSyntheticItemPayloadBytes) +
			`"}`,
	)

	normalized, err := NormalizeSyntheticItemPayload(payload)

	if !errors.Is(err, ErrItemPayloadTooLarge) {
		t.Fatalf(
			"NormalizeSyntheticItemPayload() error = %v, want %v",
			err,
			ErrItemPayloadTooLarge,
		)
	}

	if normalized != nil {
		t.Fatalf("normalized payload = %s, want nil", normalized)
	}

}

func TestNewSyntheticItemEnvelope(t *testing.T) {
	t.Parallel()

	envelope, err := NewSyntheticItemEnvelope(
		json.RawMessage(`{"token":"synthetic-token","label":"Development"}`),
	)
	if err != nil {
		t.Fatalf("NewSyntheticItemEnvelope() error = %v", err)
	}

	const wantPayload = `{"label":"Development","token":"synthetic-token"}`

	if string(envelope.Payload) != wantPayload {
		t.Fatalf("envelope payload = %s, want %s", envelope.Payload, wantPayload)
	}

	if !IsSyntheticItemNonce(envelope.Nonce) {
		t.Fatalf("envelope nonce = %q, want synthetic nonce", envelope.Nonce)
	}

}

func TestSyntheticItemNonceReturnsIndependentCopy(t *testing.T) {
	t.Parallel()

	first := SyntheticItemNonce()
	second := SyntheticItemNonce()

	if !bytes.Equal(first, second) {
		t.Fatalf("synthetic nonces differ: %q and %q", first, second)
	}

	first[0] = 'X'

	if bytes.Equal(first, second) {
		t.Fatal("SyntheticItemNonce() returned shared mutable storage")
	}

	if !IsSyntheticItemNonce(second) {
		t.Fatal("mutating one returned nonce changed the stored nonce")
	}

}

func TestItemDeleted(t *testing.T) {
	t.Parallel()

	deletedAt := time.Date(2026, time.June, 21, 20, 0, 0, 0, time.UTC)

	activeItem := Item{}
	deletedItem := Item{DeletedAt: &deletedAt}

	if activeItem.Deleted() {
		t.Fatal("active item reported itself as deleted")
	}

	if !deletedItem.Deleted() {
		t.Fatal("deleted item reported itself as active")
	}

}
