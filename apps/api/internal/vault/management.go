package vault

import (
	"context"
	"fmt"
)

func (service *Service) List(
	ctx context.Context,
	ownerID string,
) ([]Vault, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	if !service.available() {
		return nil,
			fmt.Errorf(
				"list vaults: %w",
				ErrVaultUnavailable,
			)
	}

	if !validIdentifier(ownerID) {
		return nil, ErrOwnerInvalid
	}

	storedVaults, err :=
		service.vaults.ListOwned(
			ctx,
			ownerID,
		)
	if err != nil {
		return nil,
			mapVaultOperationError(
				"list vaults",
				err,
			)
	}

	vaults := make(
		[]Vault,
		0,
		len(storedVaults),
	)

	for _, storedVault := range storedVaults {
		if !validStoredVault(
			storedVault,
			ownerID,
		) {
			return nil,
				fmt.Errorf(
					"list vaults: %w",
					ErrVaultUnavailable,
				)
		}

		vaults = append(
			vaults,
			storedVault,
		)
	}

	return vaults, nil
}

func (service *Service) Get(
	ctx context.Context,
	ownerID string,
	vaultID string,
) (Vault, error) {
	if err := ctx.Err(); err != nil {
		return Vault{}, err
	}

	if !service.available() {
		return Vault{},
			fmt.Errorf(
				"get vault: %w",
				ErrVaultUnavailable,
			)
	}

	if !validIdentifier(ownerID) {
		return Vault{}, ErrOwnerInvalid
	}

	if !validIdentifier(vaultID) {
		return Vault{}, ErrVaultNotFound
	}

	storedVault, err :=
		service.vaults.GetOwned(
			ctx,
			ownerID,
			vaultID,
		)
	if err != nil {
		return Vault{},
			mapVaultOperationError(
				"get vault",
				err,
			)
	}

	if !validStoredVault(
		storedVault,
		ownerID,
	) ||
		storedVault.ID != vaultID {
		return Vault{},
			fmt.Errorf(
				"get vault: %w",
				ErrVaultUnavailable,
			)
	}

	return storedVault, nil
}

func (service *Service) Rename(
	ctx context.Context,
	input RenameInput,
) (Vault, error) {
	if err := ctx.Err(); err != nil {
		return Vault{}, err
	}

	if !service.available() {
		return Vault{},
			fmt.Errorf(
				"rename vault: %w",
				ErrVaultUnavailable,
			)
	}

	if !validIdentifier(input.OwnerID) {
		return Vault{}, ErrOwnerInvalid
	}

	if !validIdentifier(input.VaultID) {
		return Vault{}, ErrVaultNotFound
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

	renamedVault, err :=
		service.vaults.RenameOwned(
			ctx,
			RenameStoreInput{
				OwnerID:       input.OwnerID,
				VaultID:       input.VaultID,
				Name:          normalizedName,
				CorrelationID: input.CorrelationID,
			},
		)
	if err != nil {
		return Vault{},
			mapVaultOperationError(
				"rename vault",
				err,
			)
	}

	if !validStoredVault(
		renamedVault,
		input.OwnerID,
	) ||
		renamedVault.ID != input.VaultID ||
		renamedVault.Name != normalizedName {
		return Vault{},
			fmt.Errorf(
				"rename vault: %w",
				ErrVaultUnavailable,
			)
	}

	return renamedVault, nil
}

func (service *Service) Delete(
	ctx context.Context,
	input DeleteInput,
) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	if !service.available() {
		return fmt.Errorf(
			"delete vault: %w",
			ErrVaultUnavailable,
		)
	}

	if !validIdentifier(input.OwnerID) {
		return ErrOwnerInvalid
	}

	if !validIdentifier(input.VaultID) {
		return ErrVaultNotFound
	}

	if !validIdentifier(
		input.CorrelationID,
	) {
		return ErrCorrelationIDInvalid
	}

	err := service.vaults.DeleteOwned(
		ctx,
		DeleteStoreInput(input),
	)
	if err != nil {
		return mapVaultOperationError(
			"delete vault",
			err,
		)
	}

	return nil
}
