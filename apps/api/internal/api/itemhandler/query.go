package itemhandler

import (
	"errors"
	"net/http"
	"net/url"
	"strconv"

	"github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

const maxItemRawQueryBytes = 4 * 1024

var errItemQueryInvalid = errors.New("item query is invalid")

func parseItemListOptions(
	r *http.Request,
) (vault.ItemListOptions, error) {
	values, err := parseBoundedItemQuery(r)
	if err != nil {
		return vault.ItemListOptions{},
			errItemQueryInvalid
	}

	if err := validateItemQuery(
		values,
		map[string]struct{}{
			"state": {},
			"limit": {},
			"after": {},
		},
	); err != nil {
		return vault.ItemListOptions{}, err
	}

	var options vault.ItemListOptions

	if stateValues, exists := values["state"]; exists {
		state := vault.ItemListState(stateValues[0])

		if !state.Valid() {
			return vault.ItemListOptions{},
				vault.ErrItemListStateInvalid
		}

		options.State = state
	}

	if limitValues, exists := values["limit"]; exists {
		limit, err := strconv.Atoi(limitValues[0])
		if err != nil ||
			limit < 1 ||
			limit > vault.MaxItemPageLimit {
			return vault.ItemListOptions{},
				vault.ErrItemPageLimitInvalid
		}

		options.Limit = limit
	}

	if afterValues, exists := values["after"]; exists {
		cursor, err := decodeItemCursor(afterValues[0])
		if err != nil {
			return vault.ItemListOptions{},
				vault.ErrItemCursorInvalid
		}

		options.After = &cursor
	}

	return vault.NormalizeItemListOptions(options)
}

func parseItemState(
	r *http.Request,
) (vault.ItemListState, error) {
	values, err := parseBoundedItemQuery(r)
	if err != nil {
		return "", errItemQueryInvalid
	}

	if err := validateItemQuery(
		values,
		map[string]struct{}{
			"state": {},
		},
	); err != nil {
		return "", err
	}

	stateValues, exists := values["state"]
	if !exists {
		return vault.ItemListStateActive, nil
	}

	state := vault.ItemListState(stateValues[0])

	if !state.Valid() {
		return "", vault.ErrItemListStateInvalid
	}

	return state, nil
}

func parseBoundedItemQuery(
	r *http.Request,
) (url.Values, error) {
	if r == nil ||
		r.URL == nil ||
		len(r.URL.RawQuery) >
			maxItemRawQueryBytes {
		return nil, errItemQueryInvalid
	}

	values, err := url.ParseQuery(
		r.URL.RawQuery,
	)
	if err != nil {
		return nil, errItemQueryInvalid
	}

	return values, nil
}

func validateItemQuery(
	values url.Values,
	allowedKeys map[string]struct{},
) error {
	for key, keyValues := range values {
		if _, allowed := allowedKeys[key]; !allowed {
			return errItemQueryInvalid
		}

		if len(keyValues) != 1 {
			return errItemQueryInvalid
		}
	}

	return nil
}
