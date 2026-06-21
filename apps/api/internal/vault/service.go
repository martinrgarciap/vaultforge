package vault

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Store interface {
	Create(
		ctx context.Context,
		input CreateStoreInput,
	) (Vault, error)

	ListOwned(
		ctx context.Context,
		ownerID string,
	) ([]Vault, error)

	GetOwned(
		ctx context.Context,
		ownerID string,
		vaultID string,
	) (Vault, error)

	RenameOwned(
		ctx context.Context,
		input RenameStoreInput,
	) (Vault, error)

	DeleteOwned(
		ctx context.Context,
		input DeleteStoreInput,
	) error
}

type Vault struct {
	ID            string
	OwnerID       string
	Name          string
	CryptoVersion *int16
	KDFVersion    *int16
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type CreateInput struct {
	OwnerID       string
	Name          string
	CorrelationID string
}

type CreateStoreInput struct {
	Vault         Vault
	CorrelationID string
}

type RenameInput struct {
	OwnerID       string
	VaultID       string
	Name          string
	CorrelationID string
}

type RenameStoreInput struct {
	OwnerID       string
	VaultID       string
	Name          string
	CorrelationID string
}

type DeleteInput struct {
	OwnerID       string
	VaultID       string
	CorrelationID string
}

type DeleteStoreInput struct {
	OwnerID       string
	VaultID       string
	CorrelationID string
}

type Service struct {
	vaults Store
	items  ItemStore
}

func NewService(vaults Store) *Service {
	service := &Service{vaults: vaults}

	if items, ok := vaults.(ItemStore); ok {
		service.items = items
	}

	return service
}

func (service *Service) Create(
	ctx context.Context,
	input CreateInput,
) (Vault, error) {
	if err := ctx.Err(); err != nil {
		return Vault{}, err
	}

	if !service.available() {
		return Vault{},
			fmt.Errorf(
				"create vault: %w",
				ErrVaultUnavailable,
			)
	}

	if !validIdentifier(input.OwnerID) {
		return Vault{}, ErrOwnerInvalid
	}

	if !validIdentifier(
		input.CorrelationID,
	) {
		return Vault{},
			ErrCorrelationIDInvalid
	}

	normalizedName, err := NormalizeName(
		input.Name,
	)
	if err != nil {
		return Vault{}, err
	}

	createdVault, err := service.vaults.Create(
		ctx,
		CreateStoreInput{
			Vault: Vault{
				OwnerID: input.OwnerID,
				Name:    normalizedName,
			},
			CorrelationID: input.CorrelationID,
		},
	)
	if err != nil {
		return Vault{},
			mapVaultOperationError(
				"create vault",
				err,
			)
	}

	if !validStoredVault(
		createdVault,
		input.OwnerID,
	) ||
		createdVault.Name != normalizedName {
		return Vault{},
			fmt.Errorf(
				"create vault: %w",
				ErrVaultUnavailable,
			)
	}

	return createdVault, nil
}

func (service *Service) available() bool {
	return service != nil &&
		service.vaults != nil
}

func validStoredVault(
	storedVault Vault,
	expectedOwnerID string,
) bool {
	if !validIdentifier(storedVault.ID) ||
		storedVault.OwnerID != expectedOwnerID ||
		storedVault.CreatedAt.IsZero() ||
		storedVault.UpdatedAt.IsZero() ||
		storedVault.UpdatedAt.Before(
			storedVault.CreatedAt,
		) {
		return false
	}

	normalizedName, err := NormalizeName(
		storedVault.Name,
	)
	if err != nil ||
		normalizedName != storedVault.Name {
		return false
	}

	if storedVault.CryptoVersion == nil &&
		storedVault.KDFVersion == nil {
		return true
	}

	return storedVault.CryptoVersion != nil &&
		storedVault.KDFVersion != nil &&
		*storedVault.CryptoVersion > 0 &&
		*storedVault.KDFVersion > 0
}

func validIdentifier(value string) bool {
	return value != "" &&
		!strings.ContainsAny(
			value,
			" \t\r\n",
		)
}

func mapVaultOperationError(
	operation string,
	err error,
) error {
	switch {
	case errors.Is(
		err,
		ErrVaultNotFound,
	):
		return ErrVaultNotFound

	case errors.Is(err, context.Canceled),
		errors.Is(err, context.DeadlineExceeded):
		return err

	default:
		return fmt.Errorf(
			"%s: %w",
			operation,
			ErrVaultUnavailable,
		)
	}
}
