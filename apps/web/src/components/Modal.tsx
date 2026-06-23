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

const focusableSelector = [
  "a[href]",
  "button:not([disabled])",
  "input:not([disabled]):not([type='hidden'])",
  "select:not([disabled])",
  "textarea:not([disabled])",
  "summary",
  "[contenteditable='true']",
  "[tabindex]:not([tabindex='-1'])",
].join(",");

function getFocusableElements(container: HTMLElement): HTMLElement[] {
  return Array.from(
    container.querySelectorAll<HTMLElement>(focusableSelector),
  ).filter((element) => {
    if (
      element.hidden ||
      element.matches(":disabled") ||
      element.getAttribute("aria-hidden") === "true" ||
      element.getAttribute("aria-disabled") === "true"
    ) {
      return false;
    }

    const collapsedDetails = element.closest("details:not([open])");

    return !collapsedDetails || element.tagName === "SUMMARY";
  });
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
  const dialogRef = useRef<HTMLElement | null>(null);
  const closeButtonRef = useRef<HTMLButtonElement | null>(null);

  useEffect(() => {
    const previouslyFocusedElement =
      document.activeElement instanceof HTMLElement
        ? document.activeElement
        : null;
    const previousOverflow = document.body.style.overflow;
    const dialog = dialogRef.current;

    document.body.style.overflow = "hidden";

    const initialFocus =
      closeButtonRef.current && !closeButtonRef.current.disabled
        ? closeButtonRef.current
        : dialog
          ? getFocusableElements(dialog)[0]
          : null;

    (initialFocus ?? dialog)?.focus();

    return () => {
      document.body.style.overflow = previousOverflow;

      if (previouslyFocusedElement?.isConnected) {
        previouslyFocusedElement.focus();
      }
    };
  }, []);

  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        if (!isBusy) {
          event.preventDefault();
          onClose();
        }

        return;
      }

      if (event.key !== "Tab") {
        return;
      }

      const dialog = dialogRef.current;

      if (!dialog) {
        return;
      }

      const focusableElements = getFocusableElements(dialog);

      if (focusableElements.length === 0) {
        event.preventDefault();
        dialog.focus();
        return;
      }

      const firstElement = focusableElements[0];
      const lastElement = focusableElements[focusableElements.length - 1];
      const activeElement =
        document.activeElement instanceof HTMLElement
          ? document.activeElement
          : null;
      const activeIndex = activeElement
        ? focusableElements.indexOf(activeElement)
        : -1;

      if (event.shiftKey) {
        if (activeIndex <= 0) {
          event.preventDefault();
          lastElement.focus();
        }

        return;
      }

      if (activeIndex === -1 || activeIndex === focusableElements.length - 1) {
        event.preventDefault();
        firstElement.focus();
      }
    };

    document.addEventListener("keydown", handleKeyDown);

    return () => {
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
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        tabIndex={-1}
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
