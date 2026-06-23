import type { ReactNode } from "react";

import { ApiErrorMessage } from "./ApiErrorMessage";
import { Modal } from "./Modal";

interface ConfirmModalProps {
  title: string;
  eyebrow?: string;
  children: ReactNode;
  confirmLabel: string;
  busyLabel: string;
  isBusy: boolean;
  error?: unknown;
  onClose: () => void;
  onConfirm: () => void;
}

export function ConfirmModal({
  title,
  eyebrow,
  children,
  confirmLabel,
  busyLabel,
  isBusy,
  error,
  onClose,
  onConfirm,
}: ConfirmModalProps) {
  return (
    <Modal
      title={title}
      eyebrow={eyebrow}
      onClose={onClose}
      isBusy={isBusy}
      size="small"
    >
      <div className="confirmation-copy">{children}</div>

      {error ? <ApiErrorMessage error={error} /> : null}

      <div className="modal-actions">
        <button
          className="secondary-button"
          type="button"
          onClick={onClose}
          disabled={isBusy}
        >
          Cancel
        </button>

        <button
          className="danger-button"
          type="button"
          onClick={onConfirm}
          disabled={isBusy}
        >
          {isBusy ? busyLabel : confirmLabel}
        </button>
      </div>
    </Modal>
  );
}
