package itemhandler

import (
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

func TestItemETagFormatsPositiveVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		version int
		want    string
	}{
		{
			version: 1,
			want:    `"1"`,
		},
		{
			version: 27,
			want:    `"27"`,
		},
	}

	for _, test := range tests {
		if got := itemETag(test.version); got != test.want {
			t.Fatalf(
				"itemETag(%d) = %q, want %q",
				test.version,
				got,
				test.want,
			)
		}
	}
}

func TestItemETagRejectsInvalidVersion(t *testing.T) {
	t.Parallel()

	for _, version := range []int{-1, 0} {
		if got := itemETag(version); got != "" {
			t.Fatalf(
				"itemETag(%d) = %q, want empty string",
				version,
				got,
			)
		}
	}
}

func TestExpectedItemVersionAcceptsStrongVersionETag(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(
		"PUT",
		"/v1/vaults/vault-id/items/item-id",
		nil,
	)

	request.Header.Set("If-Match", ` "12" `)

	version, err := expectedItemVersion(request)
	if err != nil {
		t.Fatalf("expectedItemVersion() error = %v", err)
	}

	if version != 12 {
		t.Fatalf("expected version = %d, want 12", version)
	}
}

func TestExpectedItemVersionRejectsMissingHeader(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(
		"PUT",
		"/v1/vaults/vault-id/items/item-id",
		nil,
	)

	_, err := expectedItemVersion(request)

	if !errors.Is(err, errIfMatchRequired) {
		t.Fatalf(
			"expectedItemVersion() error = %v, want %v",
			err,
			errIfMatchRequired,
		)
	}
}

func TestExpectedItemVersionRejectsInvalidHeaders(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
	}{
		{
			name:  "wildcard",
			value: "*",
		},
		{
			name:  "weak ETag",
			value: `W/"1"`,
		},
		{
			name:  "unquoted version",
			value: "1",
		},
		{
			name:  "zero version",
			value: `"0"`,
		},
		{
			name:  "negative version",
			value: `"-1"`,
		},
		{
			name:  "leading zero",
			value: `"01"`,
		},
		{
			name:  "multiple tags in one value",
			value: `"1", "2"`,
		},
		{
			name:  "blank quoted value",
			value: `""`,
		},
		{
			name:  "spaces inside quotes",
			value: `" 1 "`,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(
				"PUT",
				"/v1/vaults/vault-id/items/item-id",
				nil,
			)

			request.Header.Set("If-Match", test.value)

			_, err := expectedItemVersion(request)

			if !errors.Is(err, errIfMatchInvalid) {
				t.Fatalf(
					"expectedItemVersion() error = %v, want %v",
					err,
					errIfMatchInvalid,
				)
			}
		})
	}
}

func TestExpectedItemVersionRejectsMultipleHeaderValues(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(
		"PUT",
		"/v1/vaults/vault-id/items/item-id",
		nil,
	)

	request.Header.Add("If-Match", `"1"`)
	request.Header.Add("If-Match", `"2"`)

	_, err := expectedItemVersion(request)

	if !errors.Is(err, errIfMatchInvalid) {
		t.Fatalf(
			"expectedItemVersion() error = %v, want %v",
			err,
			errIfMatchInvalid,
		)
	}
}

func TestIdempotencyKeyNormalizesHeader(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(
		"POST",
		"/v1/vaults/vault-id/items",
		nil,
	)

	request.Header.Set(
		"Idempotency-Key",
		" item-create-request-123 ",
	)

	key, err := idempotencyKey(request)
	if err != nil {
		t.Fatalf("idempotencyKey() error = %v", err)
	}

	if key != "item-create-request-123" {
		t.Fatalf(
			"idempotency key = %q, want %q",
			key,
			"item-create-request-123",
		)
	}
}

func TestIdempotencyKeyAcceptsMaximumLength(t *testing.T) {
	t.Parallel()

	wantKey := strings.Repeat(
		"a",
		vault.MaxItemIdempotencyKeyBytes,
	)

	request := httptest.NewRequest(
		"POST",
		"/v1/vaults/vault-id/items",
		nil,
	)

	request.Header.Set("Idempotency-Key", wantKey)

	key, err := idempotencyKey(request)
	if err != nil {
		t.Fatalf("idempotencyKey() error = %v", err)
	}

	if key != wantKey {
		t.Fatal("maximum-length idempotency key changed")
	}
}

func TestIdempotencyKeyRejectsMissingHeader(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(
		"POST",
		"/v1/vaults/vault-id/items",
		nil,
	)

	_, err := idempotencyKey(request)

	if !errors.Is(err, errIdempotencyKeyRequired) {
		t.Fatalf(
			"idempotencyKey() error = %v, want %v",
			err,
			errIdempotencyKeyRequired,
		)
	}
}

func TestIdempotencyKeyRejectsInvalidHeaders(t *testing.T) {
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
			name:  "blank",
			value: "   ",
		},
		{
			name: "too long",
			value: strings.Repeat(
				"a",
				vault.MaxItemIdempotencyKeyBytes+1,
			),
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			request := httptest.NewRequest(
				"POST",
				"/v1/vaults/vault-id/items",
				nil,
			)

			request.Header["Idempotency-Key"] = []string{
				test.value,
			}

			_, err := idempotencyKey(request)

			if !errors.Is(err, errIdempotencyKeyInvalid) {
				t.Fatalf(
					"idempotencyKey() error = %v, want %v",
					err,
					errIdempotencyKeyInvalid,
				)
			}
		})
	}
}

func TestIdempotencyKeyRejectsMultipleValues(t *testing.T) {
	t.Parallel()

	request := httptest.NewRequest(
		"POST",
		"/v1/vaults/vault-id/items",
		nil,
	)

	request.Header.Add("Idempotency-Key", "first-key")
	request.Header.Add("Idempotency-Key", "second-key")

	_, err := idempotencyKey(request)

	if !errors.Is(err, errIdempotencyKeyInvalid) {
		t.Fatalf(
			"idempotencyKey() error = %v, want %v",
			err,
			errIdempotencyKeyInvalid,
		)
	}
}
