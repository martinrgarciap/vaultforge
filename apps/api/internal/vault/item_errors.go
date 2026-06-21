package vault

import "errors"

var (
	ErrItemPayloadEmpty          = errors.New("item payload is empty")
	ErrItemPayloadInvalid        = errors.New("item payload is invalid")
	ErrItemPayloadNotObject      = errors.New("item payload must be a JSON object")
	ErrItemPayloadTooLarge       = errors.New("item payload is too large")
	ErrItemListStateInvalid      = errors.New("item list state is invalid")
	ErrItemPageLimitInvalid      = errors.New("item page limit is invalid")
	ErrItemCursorInvalid         = errors.New("item cursor is invalid")
	ErrItemVersionInvalid        = errors.New("item version is invalid")
	ErrItemConflict              = errors.New("vault item version conflict")
	ErrItemNotFound              = errors.New("vault item was not found")
	ErrItemUnavailable           = errors.New("vault item service is unavailable")
	ErrItemIdempotencyKeyInvalid = errors.New("item idempotency key is invalid")
	ErrItemIdempotencyConflict   = errors.New("item idempotency key was reused with different input")
)
