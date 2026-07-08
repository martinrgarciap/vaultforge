import type { FormEvent } from "react";
import { useRef, useState } from "react";

import { useAuth } from "../auth/useAuth";
import { ApiErrorMessage } from "../components/ApiErrorMessage";
import { Modal } from "../components/Modal";
import { useCrypto } from "../crypto/useCrypto";
import type { ItemType, VaultItem } from "./contracts";
import { decryptItemApiResponse } from "./itemApiCrypto";
import { encryptItemWriteRequest } from "./itemEncryption";
import { ItemPayloadFields } from "./ItemPayloadFields";
import { createItemIdempotencyKey, itemCreatePath } from "./request";
import {
  defaultPayloadForType,
  formatItemPayload,
  itemTypeOptions,
  parseItemPayload,
} from "./validation";

interface ItemCreateModalProps {
  vaultId: string;
  vaultKey: Uint8Array;
  onClose: () => void;
  onCreated: (item: VaultItem) => void;
}

export function ItemCreateModal({
  vaultId,
  vaultKey,
  onClose,
  onCreated,
}: ItemCreateModalProps) {
  const { request } = useAuth();
  const { provider } = useCrypto();
  const submissionRef = useRef(false);

  const [itemType, setItemType] = useState<ItemType>("secure_note");
  const [payload, setPayload] = useState(() =>
    formatItemPayload(defaultPayloadForType("secure_note")),
  );
  const [payloadError, setPayloadError] = useState<string | undefined>();
  const [requestError, setRequestError] = useState<unknown>(null);
  const [isCreating, setIsCreating] = useState(false);

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (submissionRef.current) {
      return;
    }

    const parsedPayload = parseItemPayload(payload);

    setPayloadError(parsedPayload.ok ? undefined : parsedPayload.error);
    setRequestError(null);

    if (!parsedPayload.ok) {
      return;
    }

    submissionRef.current = true;
    setIsCreating(true);

    try {
      const encryptedRequest = await encryptItemWriteRequest({
        provider,
        vaultKey,
        type: itemType,
        payload: parsedPayload.payload,
      });

      const rawResponse = await request<unknown>(itemCreatePath(vaultId), {
        method: "POST",
        headers: {
          "Idempotency-Key": createItemIdempotencyKey(),
        },
        json: encryptedRequest,
      });

      const response = await decryptItemApiResponse({
        provider,
        vaultKey,
        value: rawResponse,
      });

      onCreated(response.item);
      onClose();
    } catch (error) {
      setRequestError(error);
    } finally {
      submissionRef.current = false;
      setIsCreating(false);
    }
  };

  return (
    <Modal
      title="Create an item"
      eyebrow="Vault item"
      onClose={onClose}
      isBusy={isCreating}
    >
      <p>
        Use synthetic development values only. The form creates the JSON payload
        automatically.
      </p>

      {requestError ? <ApiErrorMessage error={requestError} /> : null}

      <form
        className="item-form"
        onSubmit={handleSubmit}
        aria-busy={isCreating}
        noValidate
      >
        <div className="form-field">
          <label className="form-label" htmlFor="create-item-type">
            Item type
          </label>

          <select
            className="form-input"
            id="create-item-type"
            value={itemType}
            onChange={(event) => {
              const type = event.target.value as ItemType;

              setItemType(type);
              setPayload(formatItemPayload(defaultPayloadForType(type)));
              setPayloadError(undefined);
              setRequestError(null);
            }}
            disabled={isCreating}
          >
            {itemTypeOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </div>

        <ItemPayloadFields
          idPrefix="create-item"
          labelPrefix="New item"
          type={itemType}
          value={payload}
          onChange={(value) => {
            setPayload(value);
            setPayloadError(undefined);
            setRequestError(null);
          }}
          disabled={isCreating}
          describedBy={
            payloadError
              ? "create-item-help create-item-error"
              : "create-item-help"
          }
        />

        <p className="field-help" id="create-item-help">
          These fields are converted into one JSON object.
        </p>

        {payloadError ? (
          <p className="field-error" id="create-item-error">
            {payloadError}
          </p>
        ) : null}

        <details className="advanced-json">
          <summary>Advanced JSON</summary>

          <div className="form-field">
            <label className="form-label" htmlFor="create-item-payload-json">
              Item payload JSON
            </label>

            <textarea
              className="form-input json-editor"
              id="create-item-payload-json"
              value={payload}
              onChange={(event) => {
                setPayload(event.target.value);
                setPayloadError(undefined);
                setRequestError(null);
              }}
              disabled={isCreating}
              rows={10}
              spellCheck={false}
            />
          </div>
        </details>

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
            {isCreating ? "Creating item..." : "Create item"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
