import type { FormEvent, ReactNode } from "react";
import { useState } from "react";

import { useAuth } from "../auth/useAuth";
import { ApiErrorMessage } from "../components/ApiErrorMessage";
import { LoadingState, RequestErrorState } from "../components/PageState";
import { useCrypto } from "../crypto/useCrypto";
import type { Vault } from "./contracts";
import { useVaultUnlock } from "./useVaultUnlock";
import {
  isVaultCryptoInitialized,
  setupVaultCrypto,
  unlockVaultCrypto,
} from "./vaultCrypto";

const minimumVaultPassphraseLength = 8;
const maximumVaultPassphraseLength = 64;

interface VaultCryptoGateProps {
  vault: Vault;
  onVaultUpdated: (vault: Vault) => void;
  children: (vaultKey: Uint8Array) => ReactNode;
}

export function VaultCryptoGate({
  vault,
  onVaultUpdated,
  children,
}: VaultCryptoGateProps) {
  const { request } = useAuth();
  const { provider, status: cryptoStatus, error: cryptoError } = useCrypto();
  const { getVaultKey, lockVault, unlockVaultWithKey } = useVaultUnlock();

  const [passphrase, setPassphrase] = useState("");
  const [passphraseError, setPassphraseError] = useState<string>();
  const [requestError, setRequestError] = useState<unknown>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  const vaultKey = getVaultKey(vault.id);
  const cryptoInitialized = isVaultCryptoInitialized(vault);

  if (cryptoStatus === "loading") {
    return <LoadingState message="Loading browser crypto..." />;
  }

  if (cryptoStatus === "error") {
    return (
      <RequestErrorState
        error={cryptoError}
        retryLabel="Reload page"
        onRetry={() => {
          window.location.reload();
        }}
      />
    );
  }

  if (vaultKey) {
    return (
      <>
        <section className="form-notice" aria-label="Vault unlock status">
          <p>
            Vault is unlocked in browser memory. Locking the vault clears the
            in-memory key for this tab.
          </p>

          <div className="state-actions">
            <button
              className="secondary-button"
              type="button"
              onClick={() => {
                lockVault(vault.id);
              }}
            >
              Lock Vault
            </button>
          </div>
        </section>

        {children(vaultKey)}
      </>
    );
  }

  const submitLabel = cryptoInitialized
    ? "Unlock Vault"
    : "Set Up Vault Encryption";

  const busyLabel = cryptoInitialized
    ? "Unlocking Vault..."
    : "Setting Up Encryption...";

  const description = cryptoInitialized
    ? "Enter the vault passphrase to unwrap this vault key in the browser."
    : "Choose a vault passphrase. VaultForge will use it in the browser to wrap the vault key before storing metadata on the server.";

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    const normalizedPassphrase = passphrase.trim();

    setPassphraseError(undefined);
    setRequestError(null);

    if (normalizedPassphrase.length === 0) {
      setPassphraseError("Vault passphrase is required.");
      return;
    }

    if (
      !cryptoInitialized &&
      (normalizedPassphrase.length < minimumVaultPassphraseLength ||
        normalizedPassphrase.length > maximumVaultPassphraseLength)
    ) {
      setPassphraseError(
        `Vault passphrase must contain between ${minimumVaultPassphraseLength} and ${maximumVaultPassphraseLength} characters.`,
      );
      return;
    }

    setIsSubmitting(true);

    try {
      if (cryptoInitialized) {
        const unlockedKey = await unlockVaultCrypto({
          provider,
          vault,
          passphrase: normalizedPassphrase,
        });

        try {
          unlockVaultWithKey(vault.id, unlockedKey);
        } finally {
          unlockedKey.fill(0);
        }
      } else {
        const session = await setupVaultCrypto({
          provider,
          request,
          vaultId: vault.id,
          passphrase: normalizedPassphrase,
        });

        try {
          unlockVaultWithKey(vault.id, session.vaultKey);
          onVaultUpdated(session.vault);
        } finally {
          session.vaultKey.fill(0);
        }
      }

      setPassphrase("");
    } catch (error) {
      setRequestError(error);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <section className="vault-crypto-gate" aria-label="Vault encryption">
      <p className="page-kicker">Vault Encryption</p>

      <h2>{cryptoInitialized ? "Unlock Vault" : "Set Up Vault Encryption"}</h2>

      <p>{description}</p>

      <p>The passphrase and unwrapped vault key stay in browser memory.</p>

      {requestError ? <ApiErrorMessage error={requestError} /> : null}

      <form
        className="vault-form"
        onSubmit={handleSubmit}
        aria-busy={isSubmitting}
        noValidate
      >
        <div className="form-field">
          <label className="form-label" htmlFor="vault-passphrase">
            Vault passphrase
          </label>

          <input
            className="form-input"
            id="vault-passphrase"
            name="vault-passphrase"
            type="password"
            value={passphrase}
            onChange={(event) => {
              setPassphrase(event.target.value);
              setPassphraseError(undefined);
              setRequestError(null);
            }}
            autoComplete="new-password"
            disabled={isSubmitting}
            minLength={
              cryptoInitialized ? undefined : minimumVaultPassphraseLength
            }
            maxLength={
              cryptoInitialized ? undefined : maximumVaultPassphraseLength
            }
            required
            aria-invalid={passphraseError ? true : undefined}
            aria-describedby={
              passphraseError
                ? "vault-passphrase-help vault-passphrase-error"
                : "vault-passphrase-help"
            }
          />

          <p className="field-help" id="vault-passphrase-help">
            {cryptoInitialized
              ? "Enter this vault's passphrase. Use synthetic demo data only."
              : `Use between ${minimumVaultPassphraseLength} and ${maximumVaultPassphraseLength} characters. Use synthetic demo data only.`}{" "}
            This project is not audited for real credentials.
          </p>

          {passphraseError ? (
            <p className="field-error" id="vault-passphrase-error">
              {passphraseError}
            </p>
          ) : null}
        </div>

        <div className="modal-actions">
          <button
            className="primary-button"
            type="submit"
            disabled={isSubmitting}
          >
            {isSubmitting ? busyLabel : submitLabel}
          </button>
        </div>
      </form>
    </section>
  );
}
