package vault

import "errors"

var (
	ErrItemPayloadEmpty     = errors.New("item payload is empty")
	ErrItemPayloadInvalid   = errors.New("item payload is invalid")
	ErrItemPayloadNotObject = errors.New("item payload must be a JSON object")
	ErrItemPayloadTooLarge  = errors.New("item payload is too large")
	ErrItemListStateInvalid = errors.New("item list state is invalid")
	ErrItemPageLimitInvalid = errors.New("item page limit is invalid")
	ErrItemCursorInvalid    = errors.New("item cursor is invalid")
	ErrItemNotFound         = errors.New("vault item was not found")
	ErrItemUnavailable      = errors.New("vault item service is unavailable")
)
