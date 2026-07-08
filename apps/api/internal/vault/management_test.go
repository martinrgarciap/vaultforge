package vault

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"
)

const (
	managementTestOwnerID = "00000000-0000-0000-0000-000000000501"

	managementTestVaultID = "00000000-0000-0000-0000-000000000502"

	managementTestSecondVaultID = "00000000-0000-0000-0000-000000000503"

	managementTestCorrelationID = "00000000-0000-0000-0000-000000000504"
)

func TestServiceListsOwnedVaults(
	t *testing.T,
) {
	t.Parallel()

	createdAt := managementTestTime()
	updatedAt := createdAt.Add(time.Minute)

	vaultStore := &managementTestStore{
		listResult: []Vault{
			{
				ID:        managementTestVaultID,
				OwnerID:   managementTestOwnerID,
				Name:      "Development",
				CreatedAt: createdAt,
				UpdatedAt: updatedAt,
			},
			{
				ID:        managementTestSecondVaultID,
				OwnerID:   managementTestOwnerID,
				Name:      "Personal",
				CreatedAt: createdAt,
				UpdatedAt: createdAt,
			},
		},
	}

	service := NewService(vaultStore)

	vaults, err := service.List(
		context.Background(),
		managementTestOwnerID,
	)
	if err != nil {
		t.Fatalf(
			"list vaults: %v",
			err,
		)
	}

	if vaultStore.listCalls != 1 {
		t.Fatalf(
			"ListOwned() calls = %d, want 1",
			vaultStore.listCalls,
		)
	}

	if vaultStore.lastListOwnerID !=
		managementTestOwnerID {
		t.Fatalf(
			"list owner ID = %q, want %q",
			vaultStore.lastListOwnerID,
			managementTestOwnerID,
		)
	}

	if len(vaults) != 2 {
		t.Fatalf(
			"vault count = %d, want 2",
			len(vaults),
		)
	}

	if vaults[0].ID != managementTestVaultID ||
		vaults[1].ID !=
			managementTestSecondVaultID {
		t.Fatal(
			"list result did not preserve store ordering",
		)
	}
}

func TestServiceReturnsEmptyVaultList(
	t *testing.T,
) {
	t.Parallel()

	vaultStore := &managementTestStore{
		listResult: nil,
	}

	service := NewService(vaultStore)

	vaults, err := service.List(
		context.Background(),
		managementTestOwnerID,
	)
	if err != nil {
		t.Fatalf(
			"list vaults: %v",
			err,
		)
	}

	if vaults == nil {
		t.Fatal(
			"empty vault list was nil",
		)
	}

	if len(vaults) != 0 {
		t.Fatalf(
			"vault count = %d, want 0",
			len(vaults),
		)
	}
}

func TestServiceGetsOwnedVault(
	t *testing.T,
) {
	t.Parallel()

	storedVault := managementTestVault(
		managementTestVaultID,
		"Development",
	)

	vaultStore := &managementTestStore{
		getResult: storedVault,
	}

	service := NewService(vaultStore)

	result, err := service.Get(
		context.Background(),
		managementTestOwnerID,
		managementTestVaultID,
	)
	if err != nil {
		t.Fatalf(
			"get vault: %v",
			err,
		)
	}

	if vaultStore.getCalls != 1 {
		t.Fatalf(
			"GetOwned() calls = %d, want 1",
			vaultStore.getCalls,
		)
	}

	if vaultStore.lastGetOwnerID !=
		managementTestOwnerID ||
		vaultStore.lastGetVaultID !=
			managementTestVaultID {
		t.Fatal(
			"GetOwned() received incorrect ownership identifiers",
		)
	}

	if !reflect.DeepEqual(result, storedVault) {
		t.Fatalf(
			"Get() result = %+v, want %+v",
			result,
			storedVault,
		)
	}
}

func TestServiceRenameNormalizesName(
	t *testing.T,
) {
	t.Parallel()

	vaultStore := &managementTestStore{
		renameResult: managementTestVault(
			managementTestVaultID,
			"Dévelopment",
		),
	}

	service := NewService(vaultStore)

	renamedVault, err := service.Rename(
		context.Background(),
		RenameInput{
			OwnerID:       managementTestOwnerID,
			VaultID:       managementTestVaultID,
			Name:          "  De\u0301velopment  ",
			CorrelationID: managementTestCorrelationID,
		},
	)
	if err != nil {
		t.Fatalf(
			"rename vault: %v",
			err,
		)
	}

	if vaultStore.renameCalls != 1 {
		t.Fatalf(
			"RenameOwned() calls = %d, want 1",
			vaultStore.renameCalls,
		)
	}

	expectedInput := RenameStoreInput{
		OwnerID:       managementTestOwnerID,
		VaultID:       managementTestVaultID,
		Name:          "Dévelopment",
		CorrelationID: managementTestCorrelationID,
	}

	if vaultStore.lastRenameInput !=
		expectedInput {
		t.Fatalf(
			"RenameOwned() input = %+v, want %+v",
			vaultStore.lastRenameInput,
			expectedInput,
		)
	}

	if renamedVault.Name != "Dévelopment" {
		t.Fatalf(
			"renamed vault name = %q",
			renamedVault.Name,
		)
	}
}

func TestServiceDeletesOwnedVault(
	t *testing.T,
) {
	t.Parallel()

	vaultStore := &managementTestStore{}
	service := NewService(vaultStore)

	err := service.Delete(
		context.Background(),
		DeleteInput{
			OwnerID:       managementTestOwnerID,
			VaultID:       managementTestVaultID,
			CorrelationID: managementTestCorrelationID,
		},
	)
	if err != nil {
		t.Fatalf(
			"delete vault: %v",
			err,
		)
	}

	if vaultStore.deleteCalls != 1 {
		t.Fatalf(
			"DeleteOwned() calls = %d, want 1",
			vaultStore.deleteCalls,
		)
	}

	expectedInput := DeleteStoreInput{
		OwnerID:       managementTestOwnerID,
		VaultID:       managementTestVaultID,
		CorrelationID: managementTestCorrelationID,
	}

	if vaultStore.lastDeleteInput !=
		expectedInput {
		t.Fatalf(
			"DeleteOwned() input = %+v, want %+v",
			vaultStore.lastDeleteInput,
			expectedInput,
		)
	}
}

func TestServiceUsesNotFoundForInvalidVaultID(
	t *testing.T,
) {
	t.Parallel()

	vaultStore := &managementTestStore{}
	service := NewService(vaultStore)

	_, err := service.Get(
		context.Background(),
		managementTestOwnerID,
		"invalid vault ID",
	)
	if !errors.Is(err, ErrVaultNotFound) {
		t.Fatalf(
			"Get() error = %v, want %v",
			err,
			ErrVaultNotFound,
		)
	}

	err = service.Delete(
		context.Background(),
		DeleteInput{
			OwnerID:       managementTestOwnerID,
			VaultID:       "invalid vault ID",
			CorrelationID: managementTestCorrelationID,
		},
	)
	if !errors.Is(err, ErrVaultNotFound) {
		t.Fatalf(
			"Delete() error = %v, want %v",
			err,
			ErrVaultNotFound,
		)
	}

	if vaultStore.getCalls != 0 ||
		vaultStore.deleteCalls != 0 {
		t.Fatal(
			"store was called for an invalid vault ID",
		)
	}
}

func TestServiceMapsVaultNotFound(
	t *testing.T,
) {
	t.Parallel()

	vaultStore := &managementTestStore{
		getErr:    ErrVaultNotFound,
		renameErr: ErrVaultNotFound,
		deleteErr: ErrVaultNotFound,
	}

	service := NewService(vaultStore)

	_, getErr := service.Get(
		context.Background(),
		managementTestOwnerID,
		managementTestVaultID,
	)
	if !errors.Is(
		getErr,
		ErrVaultNotFound,
	) {
		t.Fatalf(
			"Get() error = %v, want %v",
			getErr,
			ErrVaultNotFound,
		)
	}

	_, renameErr := service.Rename(
		context.Background(),
		RenameInput{
			OwnerID:       managementTestOwnerID,
			VaultID:       managementTestVaultID,
			Name:          "Development",
			CorrelationID: managementTestCorrelationID,
		},
	)
	if !errors.Is(
		renameErr,
		ErrVaultNotFound,
	) {
		t.Fatalf(
			"Rename() error = %v, want %v",
			renameErr,
			ErrVaultNotFound,
		)
	}

	deleteErr := service.Delete(
		context.Background(),
		DeleteInput{
			OwnerID:       managementTestOwnerID,
			VaultID:       managementTestVaultID,
			CorrelationID: managementTestCorrelationID,
		},
	)
	if !errors.Is(
		deleteErr,
		ErrVaultNotFound,
	) {
		t.Fatalf(
			"Delete() error = %v, want %v",
			deleteErr,
			ErrVaultNotFound,
		)
	}
}

func TestServiceMapsManagementFailureSafely(
	t *testing.T,
) {
	t.Parallel()

	const internalMarker = "synthetic internal vault database detail"

	vaultStore := &managementTestStore{
		listErr: errors.New(internalMarker),
	}

	service := NewService(vaultStore)

	_, err := service.List(
		context.Background(),
		managementTestOwnerID,
	)

	if !errors.Is(
		err,
		ErrVaultUnavailable,
	) {
		t.Fatalf(
			"List() error = %v, want %v",
			err,
			ErrVaultUnavailable,
		)
	}

	if strings.Contains(
		err.Error(),
		internalMarker,
	) {
		t.Fatal(
			"vault error exposed persistence details",
		)
	}
}

func TestServiceRejectsInvalidStoredVault(
	t *testing.T,
) {
	t.Parallel()

	invalidVault := managementTestVault(
		managementTestVaultID,
		"Development",
	)
	invalidVault.OwnerID =
		"00000000-0000-0000-0000-000000000599"

	vaultStore := &managementTestStore{
		getResult: invalidVault,
	}

	service := NewService(vaultStore)

	_, err := service.Get(
		context.Background(),
		managementTestOwnerID,
		managementTestVaultID,
	)

	if !errors.Is(
		err,
		ErrVaultUnavailable,
	) {
		t.Fatalf(
			"Get() error = %v, want %v",
			err,
			ErrVaultUnavailable,
		)
	}
}

func managementTestVault(
	id string,
	name string,
) Vault {
	createdAt := managementTestTime()

	return Vault{
		ID:        id,
		OwnerID:   managementTestOwnerID,
		Name:      name,
		CreatedAt: createdAt,
		UpdatedAt: createdAt.Add(time.Minute),
	}
}

func managementTestTime() time.Time {
	return time.Date(
		2026,
		time.June,
		21,
		20,
		0,
		0,
		123456000,
		time.UTC,
	)
}

type managementTestStore struct {
	listResult   []Vault
	listErr      error
	getResult    Vault
	getErr       error
	renameResult Vault
	renameErr    error
	deleteErr    error

	listCalls   int
	getCalls    int
	renameCalls int
	deleteCalls int

	lastListOwnerID string
	lastGetOwnerID  string
	lastGetVaultID  string

	lastRenameInput RenameStoreInput
	lastDeleteInput DeleteStoreInput
}

func (*managementTestStore) Create(
	context.Context,
	CreateStoreInput,
) (Vault, error) {
	return Vault{}, nil
}

func (vaultStore *managementTestStore) ListOwned(
	_ context.Context,
	ownerID string,
) ([]Vault, error) {
	vaultStore.listCalls++
	vaultStore.lastListOwnerID = ownerID

	if vaultStore.listErr != nil {
		return nil, vaultStore.listErr
	}

	return vaultStore.listResult, nil
}

func (vaultStore *managementTestStore) GetOwned(
	_ context.Context,
	ownerID string,
	vaultID string,
) (Vault, error) {
	vaultStore.getCalls++
	vaultStore.lastGetOwnerID = ownerID
	vaultStore.lastGetVaultID = vaultID

	if vaultStore.getErr != nil {
		return Vault{}, vaultStore.getErr
	}

	return vaultStore.getResult, nil
}

func (vaultStore *managementTestStore) RenameOwned(
	_ context.Context,
	input RenameStoreInput,
) (Vault, error) {
	vaultStore.renameCalls++
	vaultStore.lastRenameInput = input

	if vaultStore.renameErr != nil {
		return Vault{},
			vaultStore.renameErr
	}

	return vaultStore.renameResult, nil
}

func (vaultStore *managementTestStore) DeleteOwned(
	_ context.Context,
	input DeleteStoreInput,
) error {
	vaultStore.deleteCalls++
	vaultStore.lastDeleteInput = input

	return vaultStore.deleteErr
}
