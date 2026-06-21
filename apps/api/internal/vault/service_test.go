package vault

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

const (
	createTestOwnerID = "00000000-0000-0000-0000-000000000301"
	createTestVaultID = "00000000-0000-0000-0000-000000000302"

	createTestCorrelationID = "00000000-0000-0000-0000-000000000303"
)

func TestServiceCreateVault(
	t *testing.T,
) {
	t.Parallel()

	createdAt := time.Date(
		2026,
		time.June,
		21,
		18,
		0,
		0,
		123456000,
		time.UTC,
	)

	vaultStore := &createTestStore{
		result: Vault{
			ID:        createTestVaultID,
			OwnerID:   createTestOwnerID,
			Name:      "Dévelopment Vault",
			CreatedAt: createdAt,
			UpdatedAt: createdAt,
		},
	}

	service := NewService(vaultStore)

	createdVault, err := service.Create(
		context.Background(),
		CreateInput{
			OwnerID:       createTestOwnerID,
			Name:          "  De\u0301velopment Vault  ",
			CorrelationID: createTestCorrelationID,
		},
	)
	if err != nil {
		t.Fatalf(
			"create vault: %v",
			err,
		)
	}

	if vaultStore.createCalls != 1 {
		t.Fatalf(
			"Create() calls = %d, want 1",
			vaultStore.createCalls,
		)
	}

	if vaultStore.lastInput.Vault.OwnerID !=
		createTestOwnerID {
		t.Fatalf(
			"stored owner ID = %q, want %q",
			vaultStore.lastInput.Vault.OwnerID,
			createTestOwnerID,
		)
	}

	if vaultStore.lastInput.Vault.Name !=
		"Dévelopment Vault" {
		t.Fatalf(
			"stored name = %q, want %q",
			vaultStore.lastInput.Vault.Name,
			"Dévelopment Vault",
		)
	}

	if vaultStore.lastInput.CorrelationID !=
		createTestCorrelationID {
		t.Fatalf(
			"stored correlation ID = %q, want %q",
			vaultStore.lastInput.CorrelationID,
			createTestCorrelationID,
		)
	}

	if vaultStore.lastInput.Vault.ID != "" {
		t.Fatal(
			"service attempted to select the vault ID",
		)
	}

	if createdVault.ID != createTestVaultID {
		t.Fatalf(
			"created vault ID = %q, want %q",
			createdVault.ID,
			createTestVaultID,
		)
	}

	if createdVault.OwnerID !=
		createTestOwnerID {
		t.Fatalf(
			"created owner ID = %q, want %q",
			createdVault.OwnerID,
			createTestOwnerID,
		)
	}

	if createdVault.Name !=
		"Dévelopment Vault" {
		t.Fatalf(
			"created name = %q",
			createdVault.Name,
		)
	}

	if !createdVault.CreatedAt.Equal(
		createdAt,
	) {
		t.Fatalf(
			"created time = %v, want %v",
			createdVault.CreatedAt,
			createdAt,
		)
	}

	if !createdVault.UpdatedAt.Equal(
		createdAt,
	) {
		t.Fatalf(
			"updated time = %v, want %v",
			createdVault.UpdatedAt,
			createdAt,
		)
	}

	if createdVault.CryptoVersion != nil ||
		createdVault.KDFVersion != nil {
		t.Fatal(
			"new dummy-data vault unexpectedly contained crypto metadata",
		)
	}
}

func TestServiceCreateRejectsInvalidInput(
	t *testing.T,
) {
	t.Parallel()

	tests := []struct {
		name      string
		input     CreateInput
		wantError error
	}{
		{
			name: "invalid owner",
			input: CreateInput{
				OwnerID:       "invalid owner",
				Name:          "Development",
				CorrelationID: createTestCorrelationID,
			},
			wantError: ErrOwnerInvalid,
		},
		{
			name: "empty vault name",
			input: CreateInput{
				OwnerID:       createTestOwnerID,
				Name:          "   ",
				CorrelationID: createTestCorrelationID,
			},
			wantError: ErrVaultNameEmpty,
		},
		{
			name: "missing correlation ID",
			input: CreateInput{
				OwnerID: createTestOwnerID,
				Name:    "Development",
			},
			wantError: ErrCorrelationIDInvalid,
		},
		{
			name: "correlation ID contains whitespace",
			input: CreateInput{
				OwnerID:       createTestOwnerID,
				Name:          "Development",
				CorrelationID: "invalid correlation ID",
			},
			wantError: ErrCorrelationIDInvalid,
		},
	}

	for _, test := range tests {
		test := test

		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			vaultStore := &createTestStore{}
			service := NewService(vaultStore)

			_, err := service.Create(
				context.Background(),
				test.input,
			)

			if !errors.Is(
				err,
				test.wantError,
			) {
				t.Fatalf(
					"Create() error = %v, want %v",
					err,
					test.wantError,
				)
			}

			if vaultStore.createCalls != 0 {
				t.Fatal(
					"store was called for invalid input",
				)
			}
		})
	}
}

func TestServiceCreateHonorsCanceledContext(
	t *testing.T,
) {
	t.Parallel()

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	vaultStore := &createTestStore{}
	service := NewService(vaultStore)

	_, err := service.Create(
		ctx,
		validCreateTestInput(),
	)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"Create() error = %v, want %v",
			err,
			context.Canceled,
		)
	}

	if vaultStore.createCalls != 0 {
		t.Fatal(
			"store was called after context cancellation",
		)
	}
}

func TestServiceCreateRejectsUnavailableStore(
	t *testing.T,
) {
	t.Parallel()

	service := NewService(nil)

	_, err := service.Create(
		context.Background(),
		validCreateTestInput(),
	)

	if !errors.Is(
		err,
		ErrVaultUnavailable,
	) {
		t.Fatalf(
			"Create() error = %v, want %v",
			err,
			ErrVaultUnavailable,
		)
	}
}

func TestServiceCreatePreservesStoreCancellation(
	t *testing.T,
) {
	t.Parallel()

	vaultStore := &createTestStore{
		err: context.DeadlineExceeded,
	}

	service := NewService(vaultStore)

	_, err := service.Create(
		context.Background(),
		validCreateTestInput(),
	)

	if !errors.Is(
		err,
		context.DeadlineExceeded,
	) {
		t.Fatalf(
			"Create() error = %v, want %v",
			err,
			context.DeadlineExceeded,
		)
	}
}

func TestServiceCreateMapsStoreFailureSafely(
	t *testing.T,
) {
	t.Parallel()

	const internalMarker = "synthetic internal database detail"

	vaultStore := &createTestStore{
		err: errors.New(internalMarker),
	}

	service := NewService(vaultStore)

	_, err := service.Create(
		context.Background(),
		validCreateTestInput(),
	)

	if !errors.Is(
		err,
		ErrVaultUnavailable,
	) {
		t.Fatalf(
			"Create() error = %v, want %v",
			err,
			ErrVaultUnavailable,
		)
	}

	if strings.Contains(
		err.Error(),
		internalMarker,
	) {
		t.Fatal(
			"create error exposed internal persistence details",
		)
	}
}

func TestServiceCreateRejectsInvalidStoreResult(
	t *testing.T,
) {
	t.Parallel()

	createdAt := time.Date(
		2026,
		time.June,
		21,
		19,
		0,
		0,
		0,
		time.UTC,
	)

	vaultStore := &createTestStore{
		result: Vault{
			OwnerID:   createTestOwnerID,
			Name:      "Development",
			CreatedAt: createdAt,
			UpdatedAt: createdAt,
		},
	}

	service := NewService(vaultStore)

	_, err := service.Create(
		context.Background(),
		validCreateTestInput(),
	)

	if !errors.Is(
		err,
		ErrVaultUnavailable,
	) {
		t.Fatalf(
			"Create() error = %v, want %v",
			err,
			ErrVaultUnavailable,
		)
	}
}

func validCreateTestInput() CreateInput {
	return CreateInput{
		OwnerID:       createTestOwnerID,
		Name:          "Development",
		CorrelationID: createTestCorrelationID,
	}
}

type createTestStore struct {
	result Vault
	err    error

	createCalls int
	lastInput   CreateStoreInput
}

func (vaultStore *createTestStore) Create(
	_ context.Context,
	input CreateStoreInput,
) (Vault, error) {
	vaultStore.createCalls++
	vaultStore.lastInput = input

	if vaultStore.err != nil {
		return Vault{},
			vaultStore.err
	}

	return vaultStore.result, nil
}

func (*createTestStore) ListOwned(
	context.Context,
	string,
) ([]Vault, error) {
	return make([]Vault, 0), nil
}

func (*createTestStore) GetOwned(
	context.Context,
	string,
	string,
) (Vault, error) {
	return Vault{}, nil
}

func (*createTestStore) RenameOwned(
	context.Context,
	RenameStoreInput,
) (Vault, error) {
	return Vault{}, nil
}

func (*createTestStore) DeleteOwned(
	context.Context,
	DeleteStoreInput,
) error {
	return nil
}
