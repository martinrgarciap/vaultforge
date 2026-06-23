import { useCallback, useEffect, useState } from "react";
import { Link, useNavigate } from "react-router";

import { useAuth } from "../auth/useAuth";
import { ApiErrorMessage } from "../components/ApiErrorMessage";
import type { Vault } from "../vaults/contracts";
import { parseVaultListResponse } from "../vaults/contracts";
import { VaultCreateModal } from "../vaults/VaultCreateModal";
import { VaultTable } from "../vaults/VaultTable";

export function VaultsPage() {
  const navigate = useNavigate();
  const { request, status } = useAuth();

  const [vaults, setVaults] = useState<Vault[]>([]);
  const [isLoading, setIsLoading] = useState(status !== "unauthenticated");
  const [loadError, setLoadError] = useState<unknown>(null);
  const [reloadVersion, setReloadVersion] = useState(0);
  const [isCreateOpen, setIsCreateOpen] = useState(false);

  useEffect(() => {
    if (status !== "authenticated") {
      return;
    }

    let active = true;

    void request<unknown>("/v1/vaults")
      .then(parseVaultListResponse)
      .then((response) => {
        if (!active) {
          return;
        }

        setVaults(response.vaults);
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
  }, [reloadVersion, request, status]);

  const closeCreateModal = useCallback(() => {
    setIsCreateOpen(false);
  }, []);

  const handleCreated = useCallback(
    (vault: Vault) => {
      navigate(`/vaults/${encodeURIComponent(vault.id)}`);
    },
    [navigate],
  );

  const refreshVaults = () => {
    setIsLoading(true);
    setLoadError(null);
    setReloadVersion((current) => current + 1);
  };

  return (
    <section className="page-card vault-page">
      <div className="page-heading-row">
        <div className="page-heading-copy">
          <p className="page-kicker">Vault Workspace</p>

          <h1>Your Vaults</h1>
        </div>

        {status === "authenticated" ? (
          <div className="page-heading-actions">
            <button
              className="secondary-button"
              type="button"
              onClick={refreshVaults}
              disabled={isLoading}
            >
              Refresh
            </button>

            <button
              className="primary-button"
              type="button"
              onClick={() => {
                setIsCreateOpen(true);
              }}
            >
              Create Vault
            </button>
          </div>
        ) : null}
      </div>

      {status === "restoring" ? (
        <p className="loading-message">Restoring your session...</p>
      ) : null}

      {status === "unauthenticated" ? (
        <div className="vault-empty">
          <p>Sign in to view and manage your vaults.</p>

          <Link className="text-link" to="/login">
            Go to login
          </Link>
        </div>
      ) : null}

      {status === "authenticated" ? (
        <section
          className="vault-list-section"
          aria-labelledby="vault-list-heading"
        >
          <h2 id="vault-list-heading">Vault List</h2>

          <p>Select a vault to view its items and settings.</p>

          {loadError ? (
            <>
              <ApiErrorMessage error={loadError} />

              <button
                className="secondary-button"
                type="button"
                onClick={refreshVaults}
              >
                Try again
              </button>
            </>
          ) : null}

          {!loadError && isLoading ? (
            <p className="loading-message">Loading vaults...</p>
          ) : null}

          {!loadError && !isLoading && vaults.length === 0 ? (
            <div className="vault-empty">
              <p>You do not have any vaults yet.</p>

              <p>Create your first synthetic-data vault.</p>
            </div>
          ) : null}

          {!loadError && !isLoading && vaults.length > 0 ? (
            <VaultTable vaults={vaults} />
          ) : null}
        </section>
      ) : null}

      {isCreateOpen ? (
        <VaultCreateModal
          onClose={closeCreateModal}
          onCreated={handleCreated}
        />
      ) : null}
    </section>
  );
}
