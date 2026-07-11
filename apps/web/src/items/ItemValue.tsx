import type { KeyboardEvent, MouseEvent } from "react";
import { useEffect, useRef, useState } from "react";

import { usePrivacy } from "../privacy/usePrivacy";

interface ItemValueProps {
  label: string;
  value: string;
  sensitive?: boolean;
  copyable?: boolean;
  compact?: boolean;
  multiline?: boolean;
  link?: boolean;
}

function safeHttpUrl(value: string): string | null {
  try {
    const url = new URL(value);

    return url.protocol === "http:" || url.protocol === "https:"
      ? url.href
      : null;
  } catch {
    return null;
  }
}

const revealDurationMs = 15 * 1000;

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

function EyeOffIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path
        d="M2.5 12s3.5-6 9.5-6c2.2 0 4 .8 5.4 1.8M21.5 12s-3.5 6-9.5 6c-2.2 0-4-.8-5.4-1.8"
        fill="none"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.8"
      />

      <path
        d="M9.9 9.9a2.8 2.8 0 0 1 4.2 4.2M3.5 3.5l17 17"
        fill="none"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
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

function CheckIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path
        d="m5 12.5 4.2 4.2L19 7"
        fill="none"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="2"
      />
    </svg>
  );
}

function ExternalLinkIcon() {
  return (
    <svg viewBox="0 0 24 24" aria-hidden="true" focusable="false">
      <path
        d="M9 6H6a2 2 0 0 0-2 2v10a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-3"
        fill="none"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.8"
      />

      <path
        d="M14 4h6v6"
        fill="none"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.8"
      />

      <path
        d="M10 14 20 4"
        fill="none"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
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
  link = false,
}: ItemValueProps) {
  const { copyValue, resetVersion } = usePrivacy();
  const [revealedResetVersion, setRevealedResetVersion] = useState<
    number | null
  >(null);
  const [copiedResetVersion, setCopiedResetVersion] = useState<number | null>(
    null,
  );
  const revealTimerRef = useRef<ReturnType<typeof window.setTimeout> | null>(
    null,
  );
  const copyFeedbackTimerRef = useRef<ReturnType<
    typeof window.setTimeout
  > | null>(null);
  const copyAttemptRef = useRef(0);

  const accessibleLabel = lowerFirst(label);
  const isRevealed = revealedResetVersion === resetVersion;
  const isCopied = copiedResetVersion === resetVersion;

  const stopClickPropagation = (
    event: MouseEvent<HTMLButtonElement | HTMLAnchorElement>,
  ) => {
    event.stopPropagation();
  };

  const stopKeyPropagation = (
    event: KeyboardEvent<HTMLButtonElement | HTMLAnchorElement>,
  ) => {
    event.stopPropagation();
  };

  useEffect(() => {
    if (revealTimerRef.current !== null) {
      window.clearTimeout(revealTimerRef.current);
      revealTimerRef.current = null;
    }
  }, [resetVersion]);

  useEffect(() => {
    return () => {
      copyAttemptRef.current += 1;

      if (revealTimerRef.current !== null) {
        window.clearTimeout(revealTimerRef.current);
        revealTimerRef.current = null;
      }

      if (copyFeedbackTimerRef.current !== null) {
        window.clearTimeout(copyFeedbackTimerRef.current);
        copyFeedbackTimerRef.current = null;
      }
    };
  }, []);

  const handleRevealToggle = () => {
    if (revealTimerRef.current !== null) {
      window.clearTimeout(revealTimerRef.current);
      revealTimerRef.current = null;
    }

    if (isRevealed) {
      setRevealedResetVersion(null);
      return;
    }

    const revealResetVersion = resetVersion;

    setRevealedResetVersion(revealResetVersion);

    revealTimerRef.current = window.setTimeout(() => {
      revealTimerRef.current = null;

      setRevealedResetVersion((current) =>
        current === revealResetVersion ? null : current,
      );
    }, revealDurationMs);
  };

  const handleCopy = async () => {
    const attemptId = copyAttemptRef.current + 1;
    const copyResetVersion = resetVersion;

    copyAttemptRef.current = attemptId;

    const didCopy = await copyValue({
      label,
      value,
    });

    if (!didCopy || copyAttemptRef.current !== attemptId) {
      return;
    }

    if (copyFeedbackTimerRef.current !== null) {
      window.clearTimeout(copyFeedbackTimerRef.current);
    }

    setCopiedResetVersion(copyResetVersion);

    copyFeedbackTimerRef.current = window.setTimeout(() => {
      copyFeedbackTimerRef.current = null;

      if (copyAttemptRef.current !== attemptId) {
        return;
      }

      setCopiedResetVersion((current) =>
        current === copyResetVersion ? null : current,
      );
    }, 1200);
  };

  const displayedValue =
    value === ""
      ? "Not provided"
      : sensitive && !isRevealed
        ? "••••••••••••"
        : value;

  const linkHref = link && !sensitive ? safeHttpUrl(displayedValue) : null;
  const valueClassName = multiline
    ? "item-value-text item-value-multiline"
    : "item-value-text";

  return (
    <div className={compact ? "item-value item-value-compact" : "item-value"}>
      <span className={valueClassName}>{displayedValue}</span>

      {value !== "" && (sensitive || copyable || linkHref) ? (
        <span className="item-value-controls">
          {sensitive ? (
            <button
              className={
                compact ? "icon-button icon-button-compact" : "icon-button"
              }
              type="button"
              onClick={(event) => {
                stopClickPropagation(event);
                handleRevealToggle();
              }}
              onKeyDown={stopKeyPropagation}
              aria-label={
                isRevealed
                  ? `Hide ${accessibleLabel}`
                  : `Show ${accessibleLabel}`
              }
              aria-pressed={isRevealed}
              title={
                isRevealed
                  ? `Hide ${label}. Automatically hides after 15 seconds.`
                  : `Show ${label}`
              }
            >
              {isRevealed ? <EyeOffIcon /> : <EyeIcon />}
            </button>
          ) : null}

          {linkHref ? (
            <a
              className={
                compact ? "icon-button icon-button-compact" : "icon-button"
              }
              href={linkHref}
              target="_blank"
              rel="noopener noreferrer"
              onClick={stopClickPropagation}
              onKeyDown={stopKeyPropagation}
              aria-label={`Open ${accessibleLabel}`}
              title={`Open ${label}`}
            >
              <ExternalLinkIcon />
            </a>
          ) : null}

          {copyable ? (
            <button
              className={
                compact ? "icon-button icon-button-compact" : "icon-button"
              }
              type="button"
              onClick={(event) => {
                stopClickPropagation(event);
                void handleCopy();
              }}
              onKeyDown={stopKeyPropagation}
              aria-label={`Copy ${accessibleLabel}`}
              title={isCopied ? `${label} copied` : `Copy ${label}`}
            >
              {isCopied ? <CheckIcon /> : <CopyIcon />}
            </button>
          ) : null}
        </span>
      ) : null}
    </div>
  );
}
