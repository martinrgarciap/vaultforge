import type { MouseEvent, ReactNode } from "react";
import { useEffect, useId, useRef } from "react";

interface ModalProps {
  title: string;
  eyebrow?: string;
  children: ReactNode;
  onClose: () => void;
  isBusy?: boolean;
  size?: "small" | "default";
}

export function Modal({
  title,
  eyebrow,
  children,
  onClose,
  isBusy = false,
  size = "default",
}: ModalProps) {
  const titleId = useId();
  const closeButtonRef = useRef<HTMLButtonElement | null>(null);

  useEffect(() => {
    const previousOverflow = document.body.style.overflow;

    document.body.style.overflow = "hidden";
    closeButtonRef.current?.focus();

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape" && !isBusy) {
        onClose();
      }
    };

    document.addEventListener("keydown", handleKeyDown);

    return () => {
      document.body.style.overflow = previousOverflow;
      document.removeEventListener("keydown", handleKeyDown);
    };
  }, [isBusy, onClose]);

  const handleBackdropMouseDown = (event: MouseEvent<HTMLDivElement>) => {
    if (event.target === event.currentTarget && !isBusy) {
      onClose();
    }
  };

  return (
    <div
      className="modal-backdrop"
      role="presentation"
      onMouseDown={handleBackdropMouseDown}
    >
      <section
        className={
          size === "small" ? "modal-panel modal-panel-small" : "modal-panel"
        }
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
      >
        <div className="modal-heading">
          <div>
            {eyebrow ? <p className="page-kicker">{eyebrow}</p> : null}

            <h2 id={titleId}>{title}</h2>
          </div>

          <button
            className="modal-close-button"
            type="button"
            ref={closeButtonRef}
            onClick={onClose}
            disabled={isBusy}
            aria-label={`Close ${title}`}
          >
            ×
          </button>
        </div>

        {children}
      </section>
    </div>
  );
}
