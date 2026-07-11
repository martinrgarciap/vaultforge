import type { FormEvent } from "react";
import { useRef, useState } from "react";

import { ApiError } from "../api/ApiError";
import { useAuth } from "../auth/useAuth";
import { ApiErrorMessage } from "../components/ApiErrorMessage";
import { Modal } from "../components/Modal";
import { useCrypto } from "../crypto/useCrypto";
import type { ItemType, VaultItem } from "./contracts";
import { decryptItemApiResponse } from "./itemApiCrypto";
import { encryptItemWriteRequest } from "./itemEncryption";
import { ItemPayloadFields } from "./ItemPayloadFields";
import {
  itemFieldValue,
  itemFormFields,
  requiredItemFieldErrors,
} from "./form";
import { itemResourcePath, itemVersionHeader } from "./request";
import {
  formatItemPayload,
  itemTypeOptions,
  parseItemPayload,
} from "./validation";

interface ItemEditModalProps {
  vaultId: string;
  vaultKey: Uint8Array;
  item: VaultItem;
  onClose: () => void;
  onUpdated: (item: VaultItem) => void;
  onConflict: (error: unknown) => void;
  openPasswordGenerator?: boolean;
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
  vaultKey,
  item,
  onClose,
  onUpdated,
  onConflict,
  openPasswordGenerator = false,
}: ItemEditModalProps) {
  const { request } = useAuth();
  const { provider } = useCrypto();
  const submissionRef = useRef(false);

  const initialPayload = formatItemPayload(item.payload);
  const [itemType, setItemType] = useState<ItemType>(item.type);
  const [payload, setPayload] = useState(initialPayload);
  const [payloadError, setPayloadError] = useState<string>();
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [requestError, setRequestError] = useState<unknown>(null);
  const [isSaving, setIsSaving] = useState(false);
  const isDirty = itemType !== item.type || payload !== initialPayload;
  const parsedPayloadForMarkers = parseItemPayload(payload);
  const changedFields =
    parsedPayloadForMarkers.ok && itemType === item.type
      ? new Set(
          itemFormFields(itemType)
            .filter(
              (field) =>
                itemFieldValue(parsedPayloadForMarkers.payload, field.key) !==
                itemFieldValue(item.payload, field.key),
            )
            .map((field) => field.key),
        )
      : new Set<string>();

  const handleSubmit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    if (submissionRef.current) {
      return;
    }

    const parsedPayload = parseItemPayload(payload);
    const nextFieldErrors = parsedPayload.ok
      ? requiredItemFieldErrors(itemType, parsedPayload.payload)
      : {};
    const hasFieldErrors = Object.keys(nextFieldErrors).length > 0;

    setPayloadError(
      parsedPayload.ok
        ? hasFieldErrors
          ? "Complete the required fields before submitting."
          : undefined
        : parsedPayload.error,
    );
    setFieldErrors(nextFieldErrors);
    setRequestError(null);

    if (!parsedPayload.ok || hasFieldErrors) {
      return;
    }

    submissionRef.current = true;
    setIsSaving(true);

    try {
      const encryptedRequest = await encryptItemWriteRequest({
        provider,
        vaultKey,
        type: itemType,
        payload: parsedPayload.payload,
      });

      const rawResponse = await request<unknown>(
        itemResourcePath(vaultId, item.id),
        {
          method: "PUT",
          headers: {
            "If-Match": itemVersionHeader(item.version),
          },
          json: encryptedRequest,
        },
      );

      const response = await decryptItemApiResponse({
        provider,
        vaultKey,
        value: rawResponse,
      });

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
            {itemType !== item.type ? (
              <span className="changed-marker">Edited</span>
            ) : null}
          </label>

          <select
            className="form-input"
            id="edit-item-type"
            value={itemType}
            onChange={(event) => {
              setItemType(event.target.value as ItemType);
              setPayloadError(undefined);
              setFieldErrors({});
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
          actionLabel="Edit item"
          type={itemType}
          value={payload}
          onChange={(value) => {
            setPayload(value);
            setPayloadError(undefined);
            setFieldErrors({});
            setRequestError(null);
          }}
          disabled={isSaving}
          describedBy={
            payloadError ? "edit-item-help edit-item-error" : "edit-item-help"
          }
          fieldErrors={fieldErrors}
          showPlaceholders={false}
          enablePasswordTools
          openPasswordGenerator={openPasswordGenerator}
          changedFields={changedFields}
        />

        <p className="field-help" id="edit-item-help">
          Fields marked with * are required.
        </p>

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
                setFieldErrors({});
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

          <button
            className="primary-button"
            type="submit"
            disabled={isSaving || !isDirty}
          >
            {isSaving ? "Saving Item..." : "Save Item"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
