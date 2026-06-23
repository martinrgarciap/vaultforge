import type { FormEvent } from "react";
import { useRef, useState } from "react";

import { useAuth } from "../auth/useAuth";
import { ApiErrorMessage } from "../components/ApiErrorMessage";
import { Modal } from "../components/Modal";
import type { Vault } from "./contracts";
import { parseVaultResponse } from "./contracts";
import {
  normalizeVaultNameForSubmission,
  validateVaultName,
} from "./validation";

interface VaultCreateModalProps {
  onClose: () => void;
  onCreated: (vault: Vault) => void;
}

export function VaultCreateModal({
  onClose,
  onCreated,
}: VaultCreateModalProps) {
  const { request } = useAuth();
  const submissionRef = useRef(false);

  const [name, setName] = useState("");
  const [nameError, setNameError] = useState<string>();
  const [requestError, setRequestError] = useState<unknown>(null);
  const [isCreating, setIsCreating] = useState(false);

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (submissionRef.current) {
      return;
    }

    const validationError = validateVaultName(name);

    setNameError(validationError);
    setRequestError(null);

    if (validationError) {
      return;
    }

    submissionRef.current = true;
    setIsCreating(true);

    try {
      const rawResponse = await request<unknown>("/v1/vaults", {
        method: "POST",
        json: {
          name: normalizeVaultNameForSubmission(name),
        },
      });

      const response = parseVaultResponse(rawResponse);

      onCreated(response.vault);
    } catch (error) {
      setRequestError(error);
    } finally {
      submissionRef.current = false;
      setIsCreating(false);
    }
  };

  return (
    <Modal
      title="Create Vault"
      eyebrow="Vault Workspace"
      onClose={onClose}
      isBusy={isCreating}
      size="small"
    >
      <p>Create a container for synthetic development secrets.</p>

      {requestError ? <ApiErrorMessage error={requestError} /> : null}

      <form
        className="vault-form"
        onSubmit={handleSubmit}
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
            value={name}
            onChange={(event) => {
              setName(event.target.value);
              setNameError(undefined);
              setRequestError(null);
            }}
            maxLength={256}
            disabled={isCreating}
            required
            aria-invalid={nameError ? true : undefined}
            aria-describedby={
              nameError
                ? "create-vault-help create-vault-error"
                : "create-vault-help"
            }
          />

          <p className="field-help" id="create-vault-help">
            Use between 1 and 128 characters.
          </p>

          {nameError ? (
            <p className="field-error" id="create-vault-error">
              {nameError}
            </p>
          ) : null}
        </div>

        <div className="modal-actions">
          <button
            className="secondary-button"
            type="button"
            onClick={onClose}
            disabled={isCreating}
          >
            Cancel
          </button>

          <button
            className="primary-button"
            type="submit"
            disabled={isCreating}
          >
            {isCreating ? "Creating Vault..." : "Create Vault"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
