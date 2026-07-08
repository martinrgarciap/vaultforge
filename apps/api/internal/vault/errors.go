package vault

import "errors"

var (
	ErrOwnerInvalid = errors.New(
		"vault owner is invalid",
	)
	ErrCorrelationIDInvalid = errors.New(
		"vault correlation ID is invalid",
	)

	ErrVaultNameInvalidUTF8 = errors.New(
		"vault name must contain valid UTF-8",
	)
	ErrVaultNameEmpty = errors.New(
		"vault name is empty",
	)
	ErrVaultNameTooLong = errors.New(
		"vault name is too long",
	)
	ErrVaultNameContainsControlCharacter = errors.New(
		"vault name contains a control character",
	)

	ErrVaultNotFound = errors.New(
		"vault was not found",
	)
	ErrVaultCryptoMetadataInvalid = errors.New(
		"vault crypto metadata is invalid",
	)
	ErrVaultCryptoMetadataAlreadyInitialized = errors.New(
		"vault crypto metadata is already initialized",
	)
	ErrVaultUnavailable = errors.New(
		"vault service is unavailable",
	)
)
