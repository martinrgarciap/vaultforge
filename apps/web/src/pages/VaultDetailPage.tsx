import type { FormEvent } from "react";
import { useEffect, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router";

import { useAuth } from "../auth/useAuth";
import { ApiErrorMessage } from "../components/ApiErrorMessage";
import type { Vault } from "../vaults/contracts";
import { parseVaultResponse } from "../vaults/contracts";
import {
  normalizeVaultNameForSubmission,
  validateVaultName,
} from "../vaults/validation";

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

  const renamingRef = useRef(false);
  const deletingRef = useRef(false);

  const [vault, setVault] = useState<Vault | null>(null);
  const [isLoading, setIsLoading] = useState(status !== "unauthenticated");
  const [loadError, setLoadError] = useState<unknown>(null);
  const [reloadVersion, setReloadVersion] = useState(0);

  const [vaultName, setVaultName] = useState("");
  const [vaultNameError, setVaultNameError] = useState<string | undefined>();
  const [renameError, setRenameError] = useState<unknown>(null);
  const [isRenaming, setIsRenaming] = useState(false);

  const [showDeleteConfirmation, setShowDeleteConfirmation] = useState(false);
  const [deleteError, setDeleteError] = useState<unknown>(null);
  const [isDeleting, setIsDeleting] = useState(false);

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
        setVaultName(response.vault.name);
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

  const handleRename = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (renamingRef.current || status !== "authenticated" || !vaultId) {
      return;
    }

    const validationError = validateVaultName(vaultName);

    setVaultNameError(validationError);
    setRenameError(null);

    if (validationError) {
      return;
    }

    const normalizedName = normalizeVaultNameForSubmission(vaultName);

    renamingRef.current = true;
    setIsRenaming(true);

    try {
      const rawResponse = await request<unknown>(
        `/v1/vaults/${encodeURIComponent(vaultId)}`,
        {
          method: "PATCH",
          json: {
            name: normalizedName,
          },
        },
      );

      const response = parseVaultResponse(rawResponse);

      setVault(response.vault);
      setVaultName(response.vault.name);
    } catch (error) {
      setRenameError(error);
    } finally {
      renamingRef.current = false;
      setIsRenaming(false);
    }
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
      setDeleteError(error);
      deletingRef.current = false;
      setIsDeleting(false);
    }
  };

  return (
    <section className="page-card vault-page">
      <p className="page-kicker">Vault workspace</p>
      <h1>Vault details</h1>

      <p>
        <Link className="text-link" to="/vaults">
          Back to vaults
        </Link>
      </p>

      {status === "restoring" ? (
        <p className="loading-message">Restoring your session...</p>
      ) : null}

      {status === "unauthenticated" ? (
        <div className="vault-empty">
          <p>Sign in to view this vault.</p>
          <Link className="text-link" to="/login">
            Go to login
          </Link>
        </div>
      ) : null}

      {status === "authenticated" && !vaultId ? (
        <div className="form-alert" role="alert">
          <p>The vault identifier is missing.</p>
        </div>
      ) : null}

      {status === "authenticated" && loadError ? (
        <>
          <ApiErrorMessage error={loadError} />

          <button
            className="secondary-button"
            type="button"
            onClick={() => {
              setIsLoading(true);
              setLoadError(null);
              setReloadVersion((current) => current + 1);
            }}
          >
            Try again
          </button>
        </>
      ) : null}

      {status === "authenticated" && !loadError && isLoading ? (
        <p className="loading-message">Loading vault...</p>
      ) : null}

      {status === "authenticated" && !loadError && !isLoading && vault ? (
        <div className="vault-detail-layout">
          <section aria-labelledby="vault-information-heading">
            <h2 id="vault-information-heading">{vault.name}</h2>

            <dl className="vault-detail-grid">
              <div>
                <dt>Vault ID</dt>
                <dd>
                  <code>{vault.id}</code>
                </dd>
              </div>

              <div>
                <dt>Created</dt>
                <dd>
                  <time dateTime={vault.createdAt}>
                    {formatTimestamp(vault.createdAt)}
                  </time>
                </dd>
              </div>

              <div>
                <dt>Last updated</dt>
                <dd>
                  <time dateTime={vault.updatedAt}>
                    {formatTimestamp(vault.updatedAt)}
                  </time>
                </dd>
              </div>

              <div>
                <dt>Crypto version</dt>
                <dd>{vault.cryptoVersion ?? "Not assigned"}</dd>
              </div>

              <div>
                <dt>KDF version</dt>
                <dd>{vault.kdfVersion ?? "Not assigned"}</dd>
              </div>
            </dl>
          </section>

          <section
            className="vault-management-card"
            aria-labelledby="rename-vault-heading"
          >
            <h2 id="rename-vault-heading">Rename vault</h2>

            {renameError ? <ApiErrorMessage error={renameError} /> : null}

            <form
              className="vault-form"
              onSubmit={handleRename}
              aria-busy={isRenaming}
              noValidate
            >
              <div className="form-field">
                <label className="form-label" htmlFor="rename-vault-name">
                  Vault name
                </label>

                <input
                  className="form-input"
                  id="rename-vault-name"
                  name="name"
                  type="text"
                  value={vaultName}
                  onChange={(event) => {
                    setVaultName(event.target.value);
                    setVaultNameError(undefined);
                    setRenameError(null);
                  }}
                  maxLength={256}
                  disabled={isRenaming || isDeleting}
                  required
                  aria-invalid={vaultNameError ? true : undefined}
                  aria-describedby={
                    vaultNameError ? "rename-vault-name-error" : undefined
                  }
                />

                {vaultNameError ? (
                  <p className="field-error" id="rename-vault-name-error">
                    {vaultNameError}
                  </p>
                ) : null}
              </div>

              <button
                className="primary-button"
                type="submit"
                disabled={isRenaming || isDeleting}
              >
                {isRenaming ? "Saving name..." : "Save name"}
              </button>
            </form>
          </section>

          <section
            className="vault-items-placeholder"
            aria-labelledby="vault-items-heading"
          >
            <h2 id="vault-items-heading">Vault items</h2>
            <p>Synthetic item workflows will be implemented during Step 7G.</p>
          </section>

          <section
            className="danger-zone"
            aria-labelledby="delete-vault-heading"
          >
            <h2 id="delete-vault-heading">Delete vault</h2>
            <p>
              Deleting this vault also removes its stored items. This action
              cannot be undone.
            </p>

            {deleteError ? <ApiErrorMessage error={deleteError} /> : null}

            {!showDeleteConfirmation ? (
              <button
                className="danger-button"
                type="button"
                onClick={() => {
                  setShowDeleteConfirmation(true);
                  setDeleteError(null);
                }}
                disabled={isRenaming}
              >
                Delete vault
              </button>
            ) : (
              <div
                className="confirmation-panel"
                role="group"
                aria-label="Delete vault confirmation"
              >
                <p>
                  Permanently delete <strong>{vault.name}</strong>?
                </p>

                <div className="button-row">
                  <button
                    className="danger-button"
                    type="button"
                    onClick={() => {
                      void handleDelete();
                    }}
                    disabled={isDeleting}
                  >
                    {isDeleting ? "Deleting vault..." : "Delete permanently"}
                  </button>

                  <button
                    className="secondary-button"
                    type="button"
                    onClick={() => {
                      setShowDeleteConfirmation(false);
                      setDeleteError(null);
                    }}
                    disabled={isDeleting}
                  >
                    Cancel
                  </button>
                </div>
              </div>
            )}
          </section>
        </div>
      ) : null}
    </section>
  );
}
