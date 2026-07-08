package vault

import (
	"bytes"
	"context"
	"fmt"
)

const (
	CurrentVaultCryptoVersion int16 = 1
	CurrentVaultKDFVersion    int16 = 1

	MinVaultCryptoSaltBytes = 16
	MaxVaultCryptoSaltBytes = 64

	MinVaultWrappedKeyBytes = 60
	MaxVaultWrappedKeyBytes = 4096
)

type VaultCryptoMetadata struct {
	CryptoVersion int16
	KDFVersion    int16
	Salt          []byte
	WrappedKey    []byte
}

type InitializeCryptoMetadataInput struct {
	OwnerID       string
	VaultID       string
	Metadata      VaultCryptoMetadata
	CorrelationID string
}

type InitializeCryptoMetadataStoreInput struct {
	OwnerID       string
	VaultID       string
	Metadata      VaultCryptoMetadata
	CorrelationID string
}

type vaultCryptoMetadataStore interface {
	InitializeCryptoMetadataOwned(
		ctx context.Context,
		input InitializeCryptoMetadataStoreInput,
	) (Vault, error)
}

func (service *Service) InitializeCryptoMetadata(
	ctx context.Context,
	input InitializeCryptoMetadataInput,
) (Vault, error) {
	if err := ctx.Err(); err != nil {
		return Vault{}, err
	}

	if !service.available() {
		return Vault{},
			fmt.Errorf(
				"initialize vault crypto metadata: %w",
				ErrVaultUnavailable,
			)
	}

	if !validIdentifier(input.OwnerID) {
		return Vault{}, ErrOwnerInvalid
	}

	if !validIdentifier(input.VaultID) {
		return Vault{}, ErrVaultNotFound
	}

	if !validIdentifier(input.CorrelationID) {
		return Vault{}, ErrCorrelationIDInvalid
	}

	if err := validateVaultCryptoMetadata(input.Metadata); err != nil {
		return Vault{}, err
	}

	store, ok := service.vaults.(vaultCryptoMetadataStore)
	if !ok {
		return Vault{},
			fmt.Errorf(
				"initialize vault crypto metadata: %w",
				ErrVaultUnavailable,
			)
	}

	initializedVault, err := store.InitializeCryptoMetadataOwned(
		ctx,
		InitializeCryptoMetadataStoreInput{
			OwnerID:       input.OwnerID,
			VaultID:       input.VaultID,
			Metadata:      cloneVaultCryptoMetadata(input.Metadata),
			CorrelationID: input.CorrelationID,
		},
	)
	if err != nil {
		return Vault{},
			mapVaultOperationError(
				"initialize vault crypto metadata",
				err,
			)
	}

	if !validStoredVault(initializedVault, input.OwnerID) ||
		initializedVault.ID != input.VaultID ||
		initializedVault.CryptoVersion == nil ||
		initializedVault.KDFVersion == nil ||
		*initializedVault.CryptoVersion != input.Metadata.CryptoVersion ||
		*initializedVault.KDFVersion != input.Metadata.KDFVersion ||
		!bytes.Equal(initializedVault.Salt, input.Metadata.Salt) ||
		!bytes.Equal(initializedVault.WrappedKey, input.Metadata.WrappedKey) {
		return Vault{},
			fmt.Errorf(
				"initialize vault crypto metadata: %w",
				ErrVaultUnavailable,
			)
	}

	return initializedVault, nil
}

func validateVaultCryptoMetadata(
	metadata VaultCryptoMetadata,
) error {
	if metadata.CryptoVersion != CurrentVaultCryptoVersion ||
		metadata.KDFVersion != CurrentVaultKDFVersion {
		return ErrVaultCryptoMetadataInvalid
	}

	if len(metadata.Salt) < MinVaultCryptoSaltBytes ||
		len(metadata.Salt) > MaxVaultCryptoSaltBytes {
		return ErrVaultCryptoMetadataInvalid
	}

	if len(metadata.WrappedKey) < MinVaultWrappedKeyBytes ||
		len(metadata.WrappedKey) > MaxVaultWrappedKeyBytes {
		return ErrVaultCryptoMetadataInvalid
	}

	return nil
}

func cloneVaultCryptoMetadata(
	metadata VaultCryptoMetadata,
) VaultCryptoMetadata {
	return VaultCryptoMetadata{
		CryptoVersion: metadata.CryptoVersion,
		KDFVersion:    metadata.KDFVersion,
		Salt:          bytes.Clone(metadata.Salt),
		WrappedKey:    bytes.Clone(metadata.WrappedKey),
	}
}
