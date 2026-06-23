package itemhandler

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseItemListOptionsRejectsOversizedRawQuery(
	t *testing.T,
) {
	t.Parallel()

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/vaults/vault-id/items",
		nil,
	)
	request.URL.RawQuery = strings.Repeat(
		"a",
		maxItemRawQueryBytes+1,
	)

	_, err := parseItemListOptions(request)

	if !errors.Is(err, errItemQueryInvalid) {
		t.Fatalf(
			"parseItemListOptions() error = %v, want %v",
			err,
			errItemQueryInvalid,
		)
	}
}

func TestParseItemStateRejectsOversizedRawQuery(
	t *testing.T,
) {
	t.Parallel()

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/vaults/vault-id/items/item-id",
		nil,
	)
	request.URL.RawQuery = strings.Repeat(
		"a",
		maxItemRawQueryBytes+1,
	)

	_, err := parseItemState(request)

	if !errors.Is(err, errItemQueryInvalid) {
		t.Fatalf(
			"parseItemState() error = %v, want %v",
			err,
			errItemQueryInvalid,
		)
	}
}

func TestItemQueryRejectsMalformedEncoding(
	t *testing.T,
) {
	t.Parallel()

	request := httptest.NewRequest(
		http.MethodGet,
		"/v1/vaults/vault-id/items",
		nil,
	)
	request.URL.RawQuery = "state=%zz"

	_, err := parseItemListOptions(request)

	if !errors.Is(err, errItemQueryInvalid) {
		t.Fatalf(
			"parseItemListOptions() error = %v, want %v",
			err,
			errItemQueryInvalid,
		)
	}
}
