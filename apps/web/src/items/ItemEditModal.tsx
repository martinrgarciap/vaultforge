import type { FormEvent } from "react";
import { useRef, useState } from "react";

import { ApiError } from "../api/ApiError";
import { useAuth } from "../auth/useAuth";
import { ApiErrorMessage } from "../components/ApiErrorMessage";
import { Modal } from "../components/Modal";
import type { ItemType, VaultItem } from "./contracts";
import { parseItemResponse } from "./contracts";
import { ItemPayloadFields } from "./ItemPayloadFields";
import { itemResourcePath, itemVersionHeader } from "./request";
import {
  formatItemPayload,
  itemTypeOptions,
  parseItemPayload,
} from "./validation";

interface ItemEditModalProps {
  vaultId: string;
  item: VaultItem;
  onClose: () => void;
  onUpdated: (item: VaultItem) => void;
  onConflict: (error: unknown) => void;
}

function isVersionConflict(error: unknown): boolean {
  return (
    error instanceof ApiError &&
    error.status === 412 &&
    error.code === "item_version_conflict"
  );
}

export function ItemEditModal({
  vaultId,
  item,
  onClose,
  onUpdated,
  onConflict,
}: ItemEditModalProps) {
  const { request } = useAuth();
  const submissionRef = useRef(false);

  const [itemType, setItemType] = useState<ItemType>(item.type);
  const [payload, setPayload] = useState(() => formatItemPayload(item.payload));
  const [payloadError, setPayloadError] = useState<string>();
  const [requestError, setRequestError] = useState<unknown>(null);
  const [isSaving, setIsSaving] = useState(false);

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
    setIsSaving(true);

    try {
      const rawResponse = await request<unknown>(
        itemResourcePath(vaultId, item.id),
        {
          method: "PUT",
          headers: {
            "If-Match": itemVersionHeader(item.version),
          },
          json: {
            type: itemType,
            payload: parsedPayload.payload,
          },
        },
      );

      const response = parseItemResponse(rawResponse);

      onUpdated(response.item);
      onClose();
    } catch (error) {
      if (isVersionConflict(error)) {
        onConflict(error);
        onClose();
        return;
      }

      setRequestError(error);
    } finally {
      submissionRef.current = false;
      setIsSaving(false);
    }
  };

  return (
    <Modal
      title="Edit Item"
      eyebrow="Item Details"
      onClose={onClose}
      isBusy={isSaving}
    >
      {requestError ? <ApiErrorMessage error={requestError} /> : null}

      <form
        className="item-form"
        onSubmit={handleSubmit}
        aria-busy={isSaving}
        noValidate
      >
        <div className="form-field">
          <label className="form-label" htmlFor="edit-item-type">
            Item type
          </label>

          <select
            className="form-input"
            id="edit-item-type"
            value={itemType}
            onChange={(event) => {
              setItemType(event.target.value as ItemType);
              setRequestError(null);
            }}
            disabled={isSaving}
          >
            {itemTypeOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </div>

        <ItemPayloadFields
          idPrefix="edit-item"
          labelPrefix="Edit item"
          type={itemType}
          value={payload}
          onChange={(value) => {
            setPayload(value);
            setPayloadError(undefined);
            setRequestError(null);
          }}
          disabled={isSaving}
          describedBy={payloadError ? "edit-item-error" : undefined}
        />

        {payloadError ? (
          <p className="field-error" id="edit-item-error">
            {payloadError}
          </p>
        ) : null}

        <details className="advanced-json">
          <summary>Advanced JSON</summary>

          <div className="form-field">
            <label className="form-label" htmlFor="edit-item-payload-json">
              Item payload JSON
            </label>

            <textarea
              className="form-input json-editor"
              id="edit-item-payload-json"
              value={payload}
              onChange={(event) => {
                setPayload(event.target.value);
                setPayloadError(undefined);
                setRequestError(null);
              }}
              disabled={isSaving}
              rows={12}
              spellCheck={false}
            />
          </div>
        </details>

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
            {isSaving ? "Saving Item..." : "Save Item"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
