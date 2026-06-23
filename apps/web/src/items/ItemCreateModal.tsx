import type { FormEvent, MouseEvent } from "react";
import { useEffect, useRef, useState } from "react";

import { useAuth } from "../auth/useAuth";
import { ApiErrorMessage } from "../components/ApiErrorMessage";
import type { ItemType, VaultItem } from "./contracts";
import { parseItemResponse } from "./contracts";
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
  onClose: () => void;
  onCreated: (item: VaultItem) => void;
}

export function ItemCreateModal({
  vaultId,
  onClose,
  onCreated,
}: ItemCreateModalProps) {
  const { request } = useAuth();

  const closeButtonRef = useRef<HTMLButtonElement | null>(null);
  const submissionRef = useRef(false);

  const [itemType, setItemType] = useState<ItemType>("secure_note");
  const [payload, setPayload] = useState(() =>
    formatItemPayload(defaultPayloadForType("secure_note")),
  );
  const [payloadError, setPayloadError] = useState<string | undefined>();
  const [requestError, setRequestError] = useState<unknown>(null);
  const [isCreating, setIsCreating] = useState(false);

  useEffect(() => {
    const previousOverflow = document.body.style.overflow;

    document.body.style.overflow = "hidden";
    closeButtonRef.current?.focus();

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !submissionRef.current) {
        onClose();
      }
    };

    document.addEventListener("keydown", handleKeyDown);

    return () => {
      document.body.style.overflow = previousOverflow;
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [onClose]);

  const handleBackdropClick = (event: MouseEvent<HTMLDivElement>) => {
    if (event.target === event.currentTarget && !submissionRef.current) {
      onClose();
    }
  };

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
      const rawResponse = await request<unknown>(itemCreatePath(vaultId), {
        method: "POST",
        headers: {
          "Idempotency-Key": createItemIdempotencyKey(),
        },
        json: {
          type: itemType,
          payload: parsedPayload.payload,
        },
      });

      const response = parseItemResponse(rawResponse);

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
    <div
      className="modal-backdrop"
      role="presentation"
      onMouseDown={handleBackdropClick}
    >
      <section
        className="modal-panel"
        role="dialog"
        aria-modal="true"
        aria-labelledby="create-item-dialog-title"
      >
        <div className="modal-heading">
          <div>
            <p className="page-kicker">Vault item</p>
            <h2 id="create-item-dialog-title">Create an item</h2>
          </div>

          <button
            className="modal-close-button"
            type="button"
            ref={closeButtonRef}
            onClick={onClose}
            disabled={isCreating}
            aria-label="Close create item dialog"
          >
            ×
          </button>
        </div>

        <p>
          Use synthetic development values only. The form creates the JSON
          payload automatically.
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
      </section>
    </div>
  );
}
