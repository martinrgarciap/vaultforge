package store

import (
	"context"
	"errors"
	"testing"
	"time"

	vaultdomain "github.com/martinrgarciap/vaultforge/apps/api/internal/vault"
)

func TestVaultStoreListOwnedFiltersAndOrdersVaults(
	t *testing.T,
) {
	vaultStore, userStore :=
		newIntegrationTestVaultStores(t)

	owner := createVaultTestUser(
		t,
		userStore,
		"vault-list-owner@example.com",
	)

	otherUser := createVaultTestUser(
		t,
		userStore,
		"vault-list-other@example.com",
	)

	olderVault := createVaultReadTestVault(
		t,
		vaultStore,
		owner.ID,
		"Older Vault",
		"vault-list-older",
	)

	newerVault := createVaultReadTestVault(
		t,
		vaultStore,
		owner.ID,
		"Newer Vault",
		"vault-list-newer",
	)

	createVaultReadTestVault(
		t,
		vaultStore,
		otherUser.ID,
		"Other User Vault",
		"vault-list-other",
	)

	olderUpdatedAt := time.Date(
		2026,
		time.June,
		21,
		15,
		0,
		0,
		0,
		time.UTC,
	)

	newerUpdatedAt := olderUpdatedAt.Add(
		time.Hour,
	)

	queryContext, cancelQuery := context.WithTimeout(
		context.Background(),
		testDatabaseTimeout,
	)
	defer cancelQuery()

	_, err := testDatabasePool.Exec(
		queryContext,
		`
			UPDATE vaults
			SET updated_at = CASE
				WHEN id = $1::uuid THEN $3
				WHEN id = $2::uuid THEN $4
				ELSE updated_at
			END
			WHERE id IN (
				$1::uuid,
				$2::uuid
			)
		`,
		olderVault.ID,
		newerVault.ID,
		olderUpdatedAt,
		newerUpdatedAt,
	)
	if err != nil {
		t.Fatal(
			"set vault list timestamps",
		)
	}

	ownedVaults, err := vaultStore.ListOwned(
		context.Background(),
		owner.ID,
	)
	if err != nil {
		t.Fatalf(
			"list owned vaults: %v",
			err,
		)
	}

	if len(ownedVaults) != 2 {
		t.Fatalf(
			"owned vault count = %d, want 2",
			len(ownedVaults),
		)
	}

	if ownedVaults[0].ID != newerVault.ID {
		t.Fatalf(
			"first vault ID = %q, want %q",
			ownedVaults[0].ID,
			newerVault.ID,
		)
	}

	if ownedVaults[1].ID != olderVault.ID {
		t.Fatalf(
			"second vault ID = %q, want %q",
			ownedVaults[1].ID,
			olderVault.ID,
		)
	}

	for _, storedVault := range ownedVaults {
		if storedVault.OwnerID != owner.ID {
			t.Fatal(
				"list returned another user's vault",
			)
		}
	}
}

func TestVaultStoreListOwnedReturnsEmptySlice(
	t *testing.T,
) {
	vaultStore, userStore :=
		newIntegrationTestVaultStores(t)

	owner := createVaultTestUser(
		t,
		userStore,
		"vault-list-empty@example.com",
	)

	ownedVaults, err := vaultStore.ListOwned(
		context.Background(),
		owner.ID,
	)
	if err != nil {
		t.Fatalf(
			"list empty owned vaults: %v",
			err,
		)
	}

	if ownedVaults == nil {
		t.Fatal(
			"empty owned vault list was nil",
		)
	}

	if len(ownedVaults) != 0 {
		t.Fatalf(
			"owned vault count = %d, want 0",
			len(ownedVaults),
		)
	}
}

func TestVaultStoreGetOwnedEnforcesOwnership(
	t *testing.T,
) {
	vaultStore, userStore :=
		newIntegrationTestVaultStores(t)

	owner := createVaultTestUser(
		t,
		userStore,
		"vault-get-owner@example.com",
	)

	otherUser := createVaultTestUser(
		t,
		userStore,
		"vault-get-other@example.com",
	)

	createdVault := createVaultReadTestVault(
		t,
		vaultStore,
		owner.ID,
		"Development",
		"vault-get-owned",
	)

	storedVault, err := vaultStore.GetOwned(
		context.Background(),
		owner.ID,
		createdVault.ID,
	)
	if err != nil {
		t.Fatalf(
			"get owned vault: %v",
			err,
		)
	}

	if storedVault.ID != createdVault.ID {
		t.Fatalf(
			"stored vault ID = %q, want %q",
			storedVault.ID,
			createdVault.ID,
		)
	}

	if storedVault.OwnerID != owner.ID {
		t.Fatalf(
			"stored owner ID = %q, want %q",
			storedVault.OwnerID,
			owner.ID,
		)
	}

	if storedVault.Name != "Development" {
		t.Fatalf(
			"stored vault name = %q, want Development",
			storedVault.Name,
		)
	}

	_, err = vaultStore.GetOwned(
		context.Background(),
		otherUser.ID,
		createdVault.ID,
	)
	if !errors.Is(
		err,
		vaultdomain.ErrVaultNotFound,
	) {
		t.Fatalf(
			"cross-user GetOwned() error = %v, want %v",
			err,
			vaultdomain.ErrVaultNotFound,
		)
	}

	_, err = vaultStore.GetOwned(
		context.Background(),
		owner.ID,
		"00000000-0000-0000-0000-000000009999",
	)
	if !errors.Is(
		err,
		vaultdomain.ErrVaultNotFound,
	) {
		t.Fatalf(
			"unknown GetOwned() error = %v, want %v",
			err,
			vaultdomain.ErrVaultNotFound,
		)
	}
}

func TestVaultStoreReadsHonorCanceledContext(
	t *testing.T,
) {
	vaultStore, userStore :=
		newIntegrationTestVaultStores(t)

	owner := createVaultTestUser(
		t,
		userStore,
		"vault-read-canceled@example.com",
	)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)
	cancel()

	_, err := vaultStore.ListOwned(
		ctx,
		owner.ID,
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"ListOwned() error = %v, want %v",
			err,
			context.Canceled,
		)
	}

	_, err = vaultStore.GetOwned(
		ctx,
		owner.ID,
		"00000000-0000-0000-0000-000000009999",
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf(
			"GetOwned() error = %v, want %v",
			err,
			context.Canceled,
		)
	}
}

func createVaultReadTestVault(
	t *testing.T,
	vaultStore *VaultStore,
	ownerID string,
	name string,
	correlationID string,
) vaultdomain.Vault {
	t.Helper()

	createdVault, err := vaultStore.Create(
		context.Background(),
		vaultdomain.CreateStoreInput{
			Vault: vaultdomain.Vault{
				OwnerID: ownerID,
				Name:    name,
			},
			CorrelationID: correlationID,
		},
	)
	if err != nil {
		t.Fatal(
			"create vault read-test fixture",
		)
	}

	return createdVault
}
