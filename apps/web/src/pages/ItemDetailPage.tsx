import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useNavigate, useParams, useSearchParams } from "react-router";

import { ApiError } from "../api/ApiError";
import { useAuth } from "../auth/useAuth";
import { ApiErrorMessage } from "../components/ApiErrorMessage";
import { BackIconLink } from "../components/BackIconLink";
import { ConfirmModal } from "../components/ConfirmModal";
import {
  EmptyState,
  LoadingState,
  RequestErrorState,
  ResourceNotFoundState,
} from "../components/PageState";
import { useCrypto } from "../crypto/useCrypto";
import type { ItemState, VaultItem } from "../items/contracts";
import { itemDisplayName } from "../items/display";
import { decryptItemApiResponse } from "../items/itemApiCrypto";
import { ItemDetails } from "../items/ItemDetails";
import { ItemEditModal } from "../items/ItemEditModal";
import {
  itemPermanentDeletePath,
  itemResourcePath,
  itemRestorePath,
  itemVersionHeader,
} from "../items/request";
import { itemTypeLabel } from "../items/validation";
import type { Vault } from "../vaults/contracts";
import { parseVaultResponse } from "../vaults/contracts";
import { useVaultUnlock } from "../vaults/useVaultUnlock";
import { VaultCryptoGate } from "../vaults/VaultCryptoGate";

function isVersionConflict(error: unknown): boolean {
  return (
    error instanceof ApiError &&
    error.status === 412 &&
    error.code === "item_version_conflict"
  );
}

export function ItemDetailPage() {
  const { vaultId, itemId } = useParams();
  const [searchParams, setSearchParams] = useSearchParams();
  const navigate = useNavigate();
  const { request, status } = useAuth();
  const { provider } = useCrypto();
  const { getVaultKey, unlockedVaultIds } = useVaultUnlock();

  const actionRef = useRef(false);

  const itemState: ItemState =
    searchParams.get("state") === "deleted" ? "deleted" : "active";

  const [vault, setVault] = useState<Vault | null>(null);
  const [item, setItem] = useState<VaultItem | null>(null);
  const [isLoading, setIsLoading] = useState(status !== "unauthenticated");
  const [loadError, setLoadError] = useState<unknown>(null);
  const [reloadVersion, setReloadVersion] = useState(0);

  const [isEditOpen, setIsEditOpen] = useState(false);
  const [openPasswordGenerator, setOpenPasswordGenerator] = useState(false);
  const [deleteMode, setDeleteMode] = useState<"soft" | "permanent" | null>(
    null,
  );

  const [actionError, setActionError] = useState<unknown>(null);
  const [isActioning, setIsActioning] = useState(false);

  useEffect(() => {
    if (status !== "authenticated" || !vaultId || !itemId) {
      return;
    }

    let active = true;

    const encodedVaultId = encodeURIComponent(vaultId);

    void request<unknown>(`/v1/vaults/${encodedVaultId}`)
      .then(parseVaultResponse)
      .then(async (vaultResponse) => {
        if (!active) {
          return;
        }

        setVault(vaultResponse.vault);

        const currentVaultKey = getVaultKey(vaultResponse.vault.id);

        if (!currentVaultKey) {
          setItem(null);
          setLoadError(null);
          setIsLoading(false);

          return;
        }

        const itemResponse = await request<unknown>(
          itemResourcePath(vaultId, itemId, itemState),
        ).then((value) =>
          decryptItemApiResponse({
            provider,
            vaultKey: currentVaultKey,
            value,
          }),
        );

        if (!active) {
          return;
        }

        setItem(itemResponse.item);
        setLoadError(null);
        setIsLoading(false);
      })
      .catch((error: unknown) => {
        if (!active) {
          return;
        }

        setLoadError(error);
        setIsLoading(false);
      });

    return () => {
      active = false;
    };
  }, [
    getVaultKey,
    itemId,
    itemState,
    provider,
    reloadVersion,
    request,
    status,
    unlockedVaultIds,
    vaultId,
  ]);

  const closeEditModal = useCallback(() => {
    setIsEditOpen(false);
    setOpenPasswordGenerator(false);
  }, []);

  const closeDeleteModal = useCallback(() => {
    setDeleteMode(null);
    setActionError(null);
  }, []);

  const handleVaultUpdated = useCallback((updatedVault: Vault) => {
    setVault(updatedVault);
  }, []);

  const reloadItem = () => {
    setIsLoading(true);
    setVault(null);
    setItem(null);
    setLoadError(null);
    setActionError(null);
    setIsEditOpen(false);
    setDeleteMode(null);
    setReloadVersion((current) => current + 1);
  };

  const beginAction = (): boolean => {
    if (actionRef.current || !item || !vaultId || !itemId) {
      return false;
    }

    actionRef.current = true;
    setIsActioning(true);
    setActionError(null);

    return true;
  };

  const finishAction = () => {
    actionRef.current = false;
    setIsActioning(false);
  };

  const handleSoftDelete = async () => {
    if (!item || !vaultId || !itemId || !beginAction()) {
      return;
    }

    try {
      const currentVaultKey = vault ? getVaultKey(vault.id) : null;

      if (!currentVaultKey) {
        setActionError(
          new Error("Unlock the vault before updating this item."),
        );
        return;
      }

      const rawResponse = await request<unknown>(
        itemResourcePath(vaultId, itemId),
        {
          method: "DELETE",
          headers: {
            "X-VaultForge-Expected-Version": itemVersionHeader(item.version),
          },
        },
      );

      const response = await decryptItemApiResponse({
        provider,
        vaultKey: currentVaultKey,
        value: rawResponse,
      });

      setItem(response.item);
      setDeleteMode(null);

      setSearchParams(
        {
          state: "deleted",
        },
        {
          replace: true,
        },
      );
    } catch (error) {
      setActionError(error);
    } finally {
      finishAction();
    }
  };

  const handleRestore = async () => {
    if (!item || !vaultId || !itemId || !beginAction()) {
      return;
    }

    try {
      const currentVaultKey = vault ? getVaultKey(vault.id) : null;

      if (!currentVaultKey) {
        setActionError(
          new Error("Unlock the vault before updating this item."),
        );
        return;
      }

      const rawResponse = await request<unknown>(
        itemRestorePath(vaultId, itemId),
        {
          method: "POST",
          headers: {
            "If-Match": itemVersionHeader(item.version),
          },
        },
      );

      const response = await decryptItemApiResponse({
        provider,
        vaultKey: currentVaultKey,
        value: rawResponse,
      });

      setItem(response.item);

      setSearchParams(
        {
          state: "active",
        },
        {
          replace: true,
        },
      );
    } catch (error) {
      setActionError(error);
    } finally {
      finishAction();
    }
  };

  const handlePermanentDelete = async () => {
    if (!item || !vaultId || !itemId || !beginAction()) {
      return;
    }

    try {
      await request<void>(itemPermanentDeletePath(vaultId, itemId), {
        method: "DELETE",
        headers: {
          "If-Match": itemVersionHeader(item.version),
        },
      });

      navigate(`/vaults/${encodeURIComponent(vaultId)}`, {
        replace: true,
      });
    } catch (error) {
      setActionError(error);
    } finally {
      finishAction();
    }
  };

  const hasVersionConflict = isVersionConflict(actionError);

  const missingResource =
    loadError instanceof ApiError && loadError.status === 404
      ? loadError.code === "vault_not_found"
        ? "vault"
        : "item"
      : null;

  const vaultPath = vaultId
    ? `/vaults/${encodeURIComponent(vaultId)}`
    : "/vaults";
  const vaultKey = vault ? getVaultKey(vault.id) : null;

  return (
    <section className="page-card vault-page">
      <BackIconLink to={vaultPath} label="Back to Vault" />

      {status === "restoring" ? (
        <>
          <p className="page-kicker">Item Details</p>

          <h1>Loading Item</h1>

          <LoadingState message="Restoring your session..." />
        </>
      ) : null}

      {status === "unauthenticated" ? (
        <>
          <p className="page-kicker">Item Details</p>

          <h1>Item Details</h1>

          <EmptyState>
            <p>Sign in to view this item.</p>

            <Link className="text-link" to="/login">
              Go to Login
            </Link>
          </EmptyState>
        </>
      ) : null}

      {status === "authenticated" && (!vaultId || !itemId) ? (
        <>
          <p className="page-kicker">Item Details</p>

          <h1>Item Details</h1>

          <div className="form-alert" role="alert">
            <p>The item identifier is missing.</p>
          </div>
        </>
      ) : null}

      {status === "authenticated" && missingResource ? (
        <>
          <p className="page-kicker">Item Details</p>

          <ResourceNotFoundState
            title={
              missingResource === "vault" ? "Vault Not Found" : "Item Not Found"
            }
            message={
              missingResource === "vault"
                ? "The vault containing this item does not exist or is no longer available."
                : "This vault item does not exist or is no longer available."
            }
            to={missingResource === "vault" ? "/vaults" : vaultPath}
            linkLabel={
              missingResource === "vault"
                ? "Return to Vault List"
                : "Return to Vault"
            }
          />
        </>
      ) : null}

      {status === "authenticated" && loadError && !missingResource ? (
        <>
          <p className="page-kicker">Item Details</p>

          <h1>Item Details</h1>

          <RequestErrorState error={loadError} onRetry={reloadItem} />
        </>
      ) : null}

      {status === "authenticated" && !loadError && isLoading ? (
        <>
          <p className="page-kicker">Item Details</p>

          <h1>Loading Item</h1>

          <LoadingState message="Loading item..." />
        </>
      ) : null}

      {vault && !vaultKey && !item && !isLoading && !loadError ? (
        <VaultCryptoGate vault={vault} onVaultUpdated={handleVaultUpdated}>
          {() => <LoadingState message="Loading item..." />}
        </VaultCryptoGate>
      ) : null}

      {status === "authenticated" &&
      !loadError &&
      !isLoading &&
      item &&
      vault ? (
        <>
          <div className="page-heading-row">
            <div className="page-heading-copy">
              <p className="page-kicker">Item Details</p>

              <h1>Item Details</h1>

              <dl className="page-entity-meta">
                <div>
                  <dt>Name</dt>
                  <dd>{itemDisplayName(item)}</dd>
                </div>
              </dl>

              <div className="item-heading-context">
                <span>
                  <span className="item-type-badge">
                    {itemTypeLabel(item.type)}
                  </span>{" "}
                  in{" "}
                  <Link className="item-vault-link" to={vaultPath}>
                    {vault.name}
                  </Link>{" "}
                  vault
                </span>

                <span
                  className={
                    item.deletedAt
                      ? "item-status item-status-deleted"
                      : "item-status item-status-active"
                  }
                >
                  {item.deletedAt ? "Deleted" : "Active"}
                </span>
              </div>
            </div>

            <div className="page-heading-actions">
              {item.deletedAt ? (
                <>
                  <button
                    className="primary-button"
                    type="button"
                    onClick={() => {
                      void handleRestore();
                    }}
                    disabled={isActioning}
                  >
                    {isActioning ? "Restoring..." : "Restore"}
                  </button>

                  <button
                    className="danger-button"
                    type="button"
                    onClick={() => {
                      setDeleteMode("permanent");
                      setActionError(null);
                    }}
                    disabled={isActioning}
                  >
                    Delete Permanently
                  </button>
                </>
              ) : (
                <>
                  <button
                    className="primary-button"
                    type="button"
                    onClick={() => {
                      setOpenPasswordGenerator(false);
                      setIsEditOpen(true);
                      setActionError(null);
                    }}
                    disabled={isActioning || !vaultKey}
                  >
                    Edit
                  </button>

                  <button
                    className="danger-button"
                    type="button"
                    onClick={() => {
                      setDeleteMode("soft");
                      setActionError(null);
                    }}
                    disabled={isActioning}
                  >
                    Delete
                  </button>
                </>
              )}
            </div>
          </div>

          <div className="item-detail-page-layout">
            <ItemDetails
              key={`${item.id}-${item.version}-${item.deletedAt ?? "active"}`}
              item={item}
              onGenerateSecurePassword={
                item.deletedAt || !vaultKey
                  ? undefined
                  : () => {
                      setOpenPasswordGenerator(true);
                      setIsEditOpen(true);
                      setActionError(null);
                    }
              }
            />

            {actionError && !deleteMode ? (
              <ApiErrorMessage error={actionError} />
            ) : null}

            {hasVersionConflict ? (
              <div className="item-conflict-panel">
                <p>
                  VaultForge will not overwrite a newer item version
                  automatically. Reload the current item before trying again.
                </p>

                <button
                  className="secondary-button"
                  type="button"
                  onClick={reloadItem}
                >
                  Reload Current Item
                </button>
              </div>
            ) : null}
          </div>

          {isEditOpen && vaultKey ? (
            <ItemEditModal
              vaultId={vault.id}
              vaultKey={vaultKey}
              item={item}
              openPasswordGenerator={openPasswordGenerator}
              onClose={closeEditModal}
              onUpdated={(updatedItem) => {
                setItem(updatedItem);
                setActionError(null);
              }}
              onConflict={(error) => {
                setActionError(error);
              }}
            />
          ) : null}

          {deleteMode === "soft" ? (
            <ConfirmModal
              title="Delete Item?"
              eyebrow="Item Details"
              confirmLabel="Delete"
              busyLabel="Deleting..."
              isBusy={isActioning}
              error={actionError}
              onClose={closeDeleteModal}
              onConfirm={() => {
                void handleSoftDelete();
              }}
            >
              <p>
                Delete <strong>{itemDisplayName(item)}</strong>?
              </p>

              <p>
                The item will be moved to Deleted Items and can be restored
                later.
              </p>
            </ConfirmModal>
          ) : null}

          {deleteMode === "permanent" ? (
            <ConfirmModal
              title="Permanently Delete Item?"
              eyebrow="Item Details"
              confirmLabel="Delete Permanently"
              busyLabel="Deleting Permanently..."
              isBusy={isActioning}
              error={actionError}
              onClose={closeDeleteModal}
              onConfirm={() => {
                void handlePermanentDelete();
              }}
            >
              <p>
                Permanently delete <strong>{itemDisplayName(item)}</strong>?
              </p>

              <p>This action cannot be undone.</p>
            </ConfirmModal>
          ) : null}
        </>
      ) : null}
    </section>
  );
}
