import type { FormEvent } from "react";
import { useEffect, useRef, useState } from "react";
import { Link } from "react-router";

import { useAuth } from "../auth/useAuth";
import { ApiErrorMessage } from "../components/ApiErrorMessage";
import type { Vault } from "../vaults/contracts";
import {
  parseVaultListResponse,
  parseVaultResponse,
} from "../vaults/contracts";
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

export function VaultsPage() {
  const { request, status } = useAuth();
  const creatingRef = useRef(false);

  const [vaults, setVaults] = useState<Vault[]>([]);
  const [isLoading, setIsLoading] = useState(status !== "unauthenticated");
  const [loadError, setLoadError] = useState<unknown>(null);
  const [reloadVersion, setReloadVersion] = useState(0);

  const [vaultName, setVaultName] = useState("");
  const [vaultNameError, setVaultNameError] = useState<string | undefined>();
  const [createError, setCreateError] = useState<unknown>(null);
  const [isCreating, setIsCreating] = useState(false);

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

  const handleCreate = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (creatingRef.current || status !== "authenticated") {
      return;
    }

    const validationError = validateVaultName(vaultName);

    setVaultNameError(validationError);
    setCreateError(null);

    if (validationError) {
      return;
    }

    const normalizedName = normalizeVaultNameForSubmission(vaultName);

    creatingRef.current = true;
    setIsCreating(true);

    try {
      const rawResponse = await request<unknown>("/v1/vaults", {
        method: "POST",
        json: {
          name: normalizedName,
        },
      });

      const response = parseVaultResponse(rawResponse);

      setVaults((currentVaults) => [response.vault, ...currentVaults]);
      setVaultName("");
    } catch (error) {
      setCreateError(error);
    } finally {
      creatingRef.current = false;
      setIsCreating(false);
    }
  };

  return (
    <section className="page-card vault-page">
      <p className="page-kicker">Vault workspace</p>
      <h1>Your vaults</h1>

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
        <div className="vault-layout">
          <section
            className="vault-create-card"
            aria-labelledby="create-vault-heading"
          >
            <h2 id="create-vault-heading">Create a vault</h2>
            <p>Create a container for synthetic development secrets.</p>

            {createError ? <ApiErrorMessage error={createError} /> : null}

            <form
              className="vault-form"
              onSubmit={handleCreate}
              aria-busy={isCreating}
              noValidate
            >
              <div className="form-field">
                <label className="form-label" htmlFor="create-vault-name">
                  Vault name
                </label>

                <input
                  className="form-input"
                  id="create-vault-name"
                  name="name"
                  type="text"
                  value={vaultName}
                  onChange={(event) => {
                    setVaultName(event.target.value);
                    setVaultNameError(undefined);
                    setCreateError(null);
                  }}
                  maxLength={256}
                  disabled={isCreating}
                  required
                  aria-invalid={vaultNameError ? true : undefined}
                  aria-describedby={
                    vaultNameError
                      ? "create-vault-name-help create-vault-name-error"
                      : "create-vault-name-help"
                  }
                />

                <p className="field-help" id="create-vault-name-help">
                  Use between 1 and 128 characters.
                </p>

                {vaultNameError ? (
                  <p className="field-error" id="create-vault-name-error">
                    {vaultNameError}
                  </p>
                ) : null}
              </div>

              <button
                className="primary-button"
                type="submit"
                disabled={isCreating}
              >
                {isCreating ? "Creating vault..." : "Create vault"}
              </button>
            </form>
          </section>

          <section aria-labelledby="vault-list-heading">
            <div className="section-heading-row">
              <div>
                <h2 id="vault-list-heading">Vault list</h2>
                <p>Open a vault to rename it or continue to its items.</p>
              </div>

              <button
                className="secondary-button"
                type="button"
                onClick={() => {
                  setIsLoading(true);
                  setLoadError(null);
                  setReloadVersion((current) => current + 1);
                }}
                disabled={isLoading}
              >
                Refresh
              </button>
            </div>

            {loadError ? (
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

            {!loadError && isLoading ? (
              <p className="loading-message">Loading vaults...</p>
            ) : null}

            {!loadError && !isLoading && vaults.length === 0 ? (
              <div className="vault-empty">
                <p>You do not have any vaults yet.</p>
                <p>Create your first synthetic-data vault above.</p>
              </div>
            ) : null}

            {!loadError && !isLoading && vaults.length > 0 ? (
              <ul className="vault-list">
                {vaults.map((vault) => (
                  <li className="vault-list-item" key={vault.id}>
                    <div>
                      <Link
                        className="vault-link"
                        to={`/vaults/${encodeURIComponent(vault.id)}`}
                      >
                        {vault.name}
                      </Link>

                      <p className="vault-meta">
                        Updated{" "}
                        <time dateTime={vault.updatedAt}>
                          {formatTimestamp(vault.updatedAt)}
                        </time>
                      </p>
                    </div>

                    <Link
                      className="secondary-link"
                      to={`/vaults/${encodeURIComponent(vault.id)}`}
                    >
                      Open
                    </Link>
                  </li>
                ))}
              </ul>
            ) : null}
          </section>
        </div>
      ) : null}
    </section>
  );
}
