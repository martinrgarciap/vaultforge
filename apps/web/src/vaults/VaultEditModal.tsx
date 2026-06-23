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

interface VaultEditModalProps {
  vault: Vault;
  onClose: () => void;
  onUpdated: (vault: Vault) => void;
}

export function VaultEditModal({
  vault,
  onClose,
  onUpdated,
}: VaultEditModalProps) {
  const { request } = useAuth();
  const submissionRef = useRef(false);

  const [name, setName] = useState(vault.name);
  const [nameError, setNameError] = useState<string>();
  const [requestError, setRequestError] = useState<unknown>(null);
  const [isSaving, setIsSaving] = useState(false);

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
    setIsSaving(true);

    try {
      const rawResponse = await request<unknown>(
        `/v1/vaults/${encodeURIComponent(vault.id)}`,
        {
          method: "PATCH",
          json: {
            name: normalizeVaultNameForSubmission(name),
          },
        },
      );

      const response = parseVaultResponse(rawResponse);

      onUpdated(response.vault);
      onClose();
    } catch (error) {
      setRequestError(error);
    } finally {
      submissionRef.current = false;
      setIsSaving(false);
    }
  };

  return (
    <Modal
      title="Edit Vault"
      eyebrow="Vault Workspace"
      onClose={onClose}
      isBusy={isSaving}
      size="small"
    >
      {requestError ? <ApiErrorMessage error={requestError} /> : null}

      <form
        className="vault-form"
        onSubmit={handleSubmit}
        aria-busy={isSaving}
        noValidate
      >
        <div className="form-field">
          <label className="form-label" htmlFor="edit-vault-name">
            Vault name
          </label>

          <input
            className="form-input"
            id="edit-vault-name"
            name="name"
            type="text"
            value={name}
            onChange={(event) => {
              setName(event.target.value);
              setNameError(undefined);
              setRequestError(null);
            }}
            maxLength={256}
            disabled={isSaving}
            required
            aria-invalid={nameError ? true : undefined}
            aria-describedby={nameError ? "edit-vault-error" : undefined}
          />

          {nameError ? (
            <p className="field-error" id="edit-vault-error">
              {nameError}
            </p>
          ) : null}
        </div>

        <div className="modal-actions">
          <button
            className="secondary-button"
            type="button"
            onClick={onClose}
            disabled={isSaving}
          >
            Cancel
          </button>

          <button className="primary-button" type="submit" disabled={isSaving}>
            {isSaving ? "Saving Changes..." : "Save Changes"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
