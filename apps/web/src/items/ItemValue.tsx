import type { KeyboardEvent, MouseEvent } from "react";
import { useState } from "react";

interface ItemValueProps {
  label: string;
  value: string;
  sensitive?: boolean;
  copyable?: boolean;
  compact?: boolean;
  multiline?: boolean;
}

function EyeIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path
        d="M2.5 12s3.5-6 9.5-6 9.5 6 9.5 6-3.5 6-9.5 6-9.5-6-9.5-6Z"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.8"
      />

      <circle
        cx="12"
        cy="12"
        r="2.8"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.8"
      />
    </svg>
  );
}

function CopyIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <rect
        x="8"
        y="8"
        width="11"
        height="11"
        rx="2"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.8"
      />

      <path
        d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.8"
      />
    </svg>
  );
}

function lowerFirst(value: string): string {
  return value.charAt(0).toLowerCase() + value.slice(1);
}

export function ItemValue({
  label,
  value,
  sensitive = false,
  copyable = false,
  compact = false,
  multiline = false,
}: ItemValueProps) {
  const [isRevealed, setIsRevealed] = useState(false);
  const [copyStatus, setCopyStatus] = useState("");

  const accessibleLabel = lowerFirst(label);

  const stopClickPropagation = (event: MouseEvent<HTMLButtonElement>) => {
    event.stopPropagation();
  };

  const stopKeyPropagation = (event: KeyboardEvent<HTMLButtonElement>) => {
    event.stopPropagation();
  };

  const copyValue = async () => {
    if (!navigator.clipboard?.writeText) {
      setCopyStatus("Clipboard access is unavailable.");
      return;
    }

    try {
      await navigator.clipboard.writeText(value);
      setCopyStatus(`${label} copied.`);
    } catch {
      setCopyStatus("The value could not be copied.");
    }
  };

  const displayedValue =
    value === ""
      ? "Not provided"
      : sensitive && !isRevealed
        ? "••••••••••••"
        : value;

  return (
    <div className={compact ? "item-value item-value-compact" : "item-value"}>
      <span
        className={
          multiline ? "item-value-text item-value-multiline" : "item-value-text"
        }
      >
        {displayedValue}
      </span>

      {value !== "" && (sensitive || copyable) ? (
        <span className="item-value-controls">
          {sensitive ? (
            <button
              className={
                compact ? "icon-button icon-button-compact" : "icon-button"
              }
              type="button"
              onClick={(event) => {
                stopClickPropagation(event);
                setIsRevealed((current) => !current);
              }}
              onKeyDown={stopKeyPropagation}
              aria-label={
                isRevealed
                  ? `Hide ${accessibleLabel}`
                  : `Show ${accessibleLabel}`
              }
              title={isRevealed ? `Hide ${label}` : `Show ${label}`}
            >
              <EyeIcon />
            </button>
          ) : null}

          {copyable ? (
            <button
              className={
                compact ? "icon-button icon-button-compact" : "icon-button"
              }
              type="button"
              onClick={(event) => {
                stopClickPropagation(event);
                void copyValue();
              }}
              onKeyDown={stopKeyPropagation}
              aria-label={`Copy ${accessibleLabel}`}
              title={`Copy ${label}`}
            >
              <CopyIcon />
            </button>
          ) : null}
        </span>
      ) : null}

      <span className="visually-hidden" aria-live="polite">
        {copyStatus}
      </span>
    </div>
  );
}
