import { useCallback, useEffect, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router";

import { ApiError } from "../api/ApiError";
import { useAuth } from "../auth/useAuth";
import { BackIconLink } from "../components/BackIconLink";
import { ConfirmModal } from "../components/ConfirmModal";
import {
  EmptyState,
  LoadingState,
  RequestErrorState,
  ResourceNotFoundState,
} from "../components/PageState";
import { ItemWorkspace } from "../items/ItemWorkspace";
import type { Vault } from "../vaults/contracts";
import { parseVaultResponse } from "../vaults/contracts";
import { VaultCryptoGate } from "../vaults/VaultCryptoGate";
import { VaultEditModal } from "../vaults/VaultEditModal";

function formatTimestamp(value: string): string {
  const timestamp = new Date(value);

  if (Number.isNaN(timestamp.getTime())) {
    return value;
  }

  return timestamp.toLocaleString();
}

export function VaultDetailPage() {
  const { vaultId } = useParams();
  const navigate = useNavigate();
  const { request, status } = useAuth();

  const deletingRef = useRef(false);

  const [vault, setVault] = useState<Vault | null>(null);
  const [isLoading, setIsLoading] = useState(status !== "unauthenticated");
  const [loadError, setLoadError] = useState<unknown>(null);
  const [reloadVersion, setReloadVersion] = useState(0);

  const [isEditOpen, setIsEditOpen] = useState(false);
  const [isDeleteOpen, setIsDeleteOpen] = useState(false);
  const [deleteError, setDeleteError] = useState<unknown>(null);
  const [isDeleting, setIsDeleting] = useState(false);

  const vaultNotFound =
    loadError instanceof ApiError &&
    loadError.status === 404 &&
    loadError.code === "vault_not_found";

  useEffect(() => {
    if (status !== "authenticated" || !vaultId) {
      return;
    }

    let active = true;

    void request<unknown>(`/v1/vaults/${encodeURIComponent(vaultId)}`)
      .then(parseVaultResponse)
      .then((response) => {
        if (!active) {
          return;
        }

        setVault(response.vault);
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
  }, [reloadVersion, request, status, vaultId]);

  const closeEditModal = useCallback(() => {
    setIsEditOpen(false);
  }, []);

  const closeDeleteModal = useCallback(() => {
    setIsDeleteOpen(false);
    setDeleteError(null);
  }, []);

  const handleVaultUpdated = useCallback((updatedVault: Vault) => {
    setVault(updatedVault);
  }, []);

  const reloadVault = () => {
    setIsLoading(true);
    setLoadError(null);
    setReloadVersion((current) => current + 1);
  };

  const handleDelete = async () => {
    if (deletingRef.current || status !== "authenticated" || !vaultId) {
      return;
    }

    deletingRef.current = true;
    setIsDeleting(true);
    setDeleteError(null);

    try {
      await request<void>(`/v1/vaults/${encodeURIComponent(vaultId)}`, {
        method: "DELETE",
      });

      navigate("/vaults", {
        replace: true,
      });
    } catch (error) {
      if (error instanceof ApiError && error.status === 404) {
        navigate("/vaults", {
          replace: true,
        });

        return;
      }

      setDeleteError(error);
      deletingRef.current = false;
      setIsDeleting(false);
    }
  };

  return (
    <section className="page-card vault-page">
      <BackIconLink to="/vaults" label="Back to Vault List" />

      {status === "restoring" ? (
        <>
          <p className="page-kicker">Vault Workspace</p>

          <h1>Vault Details</h1>

          <LoadingState message="Restoring your session..." />
        </>
      ) : null}

      {status === "unauthenticated" ? (
        <>
          <p className="page-kicker">Vault Workspace</p>

          <h1>Vault Details</h1>

          <EmptyState>
            <p>Sign in to view this vault.</p>

            <Link className="text-link" to="/login">
              Go to Login
            </Link>
          </EmptyState>
        </>
      ) : null}

      {status === "authenticated" && !vaultId ? (
        <>
          <p className="page-kicker">Vault Workspace</p>

          <h1>Vault Details</h1>

          <div className="form-alert" role="alert">
            <p>The vault identifier is missing.</p>
          </div>
        </>
      ) : null}

      {status === "authenticated" && vaultNotFound ? (
        <>
          <p className="page-kicker">Vault Workspace</p>

          <ResourceNotFoundState
            title="Vault Not Found"
            message="This vault does not exist or is no longer available."
            to="/vaults"
            linkLabel="Return to Vault List"
          />
        </>
      ) : null}

      {status === "authenticated" && loadError && !vaultNotFound ? (
        <>
          <p className="page-kicker">Vault Workspace</p>

          <h1>Vault Details</h1>

          <RequestErrorState error={loadError} onRetry={reloadVault} />
        </>
      ) : null}

      {status === "authenticated" && !loadError && isLoading ? (
        <>
          <p className="page-kicker">Vault Workspace</p>

          <h1>Vault Details</h1>

          <LoadingState message="Loading vault..." />
        </>
      ) : null}

      {status === "authenticated" && !loadError && !isLoading && vault ? (
        <>
          <div className="page-heading-row">
            <div className="page-heading-copy">
              <p className="page-kicker">Vault Workspace</p>

              <h1>Vault Details</h1>

              <p className="page-entity-name">{vault.name}</p>
            </div>

            <div className="page-heading-actions">
              <button
                className="primary-button"
                type="button"
                onClick={() => {
                  setIsEditOpen(true);
                }}
              >
                Edit
              </button>

              <button
                className="danger-button"
                type="button"
                onClick={() => {
                  setIsDeleteOpen(true);
                  setDeleteError(null);
                }}
              >
                Delete
              </button>
            </div>
          </div>

          <dl className="vault-summary-grid">
            <div>
              <dt>Created</dt>

              <dd>
                <time dateTime={vault.createdAt}>
                  {formatTimestamp(vault.createdAt)}
                </time>
              </dd>
            </div>

            <div>
              <dt>Last Updated</dt>

              <dd>
                <time dateTime={vault.updatedAt}>
                  {formatTimestamp(vault.updatedAt)}
                </time>
              </dd>
            </div>
          </dl>

          <div className="vault-detail-content">
            <VaultCryptoGate vault={vault} onVaultUpdated={handleVaultUpdated}>
              {(vaultKey) => (
                <ItemWorkspace
                  key={vault.id}
                  vaultId={vault.id}
                  vaultKey={vaultKey}
                />
              )}
            </VaultCryptoGate>
          </div>

          {isEditOpen ? (
            <VaultEditModal
              vault={vault}
              onClose={closeEditModal}
              onUpdated={handleVaultUpdated}
            />
          ) : null}

          {isDeleteOpen ? (
            <ConfirmModal
              title="Delete Vault?"
              eyebrow="Vault Workspace"
              confirmLabel="Delete Vault"
              busyLabel="Deleting Vault..."
              isBusy={isDeleting}
              error={deleteError}
              onClose={closeDeleteModal}
              onConfirm={() => {
                void handleDelete();
              }}
            >
              <p>
                Delete <strong>{vault.name}</strong>?
              </p>

              <p>
                All items inside this vault will also be deleted. This action
                cannot be undone.
              </p>
            </ConfirmModal>
          ) : null}
        </>
      ) : null}
    </section>
  );
}
