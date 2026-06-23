import type { ReactNode } from "react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { useAuth } from "../auth/useAuth";
import type { CopyValueInput, PrivacyContextValue } from "./PrivacyContext";
import { PrivacyContext } from "./PrivacyContext";

interface PrivacyProviderProps {
  children: ReactNode;
  clipboardClearDelayMs?: number;
  inactivityDelayMs?: number;
}

const defaultClipboardClearDelayMs = 30 * 1000;
const defaultInactivityDelayMs = 5 * 60 * 1000;
const copyToastDurationMs = 2 * 1000;

const inactivityResetMessage =
  "Sensitive values were hidden after inactivity. This does not cryptographically lock the vault.";

const hiddenTabResetMessage =
  "Sensitive values were hidden because the tab was hidden. This does not cryptographically lock the vault.";

export function PrivacyProvider({
  children,
  clipboardClearDelayMs = defaultClipboardClearDelayMs,
  inactivityDelayMs = defaultInactivityDelayMs,
}: PrivacyProviderProps) {
  const { status } = useAuth();

  const [resetVersion, setResetVersion] = useState(0);
  const [announcement, setAnnouncement] = useState("");
  const [copyToast, setCopyToast] = useState("");

  const clipboardTimerRef = useRef<ReturnType<typeof window.setTimeout> | null>(
    null,
  );
  const inactivityTimerRef = useRef<ReturnType<
    typeof window.setTimeout
  > | null>(null);
  const copyToastTimerRef = useRef<ReturnType<typeof window.setTimeout> | null>(
    null,
  );

  const clipboardOperationRef = useRef(0);
  const clipboardWriteQueueRef = useRef<Promise<void>>(Promise.resolve());
  const mountedRef = useRef(false);
  const previousStatusRef = useRef(status);

  const announce = useCallback((message: string) => {
    if (mountedRef.current) {
      setAnnouncement(message);
    }
  }, []);

  const showCopyToast = useCallback((message: string) => {
    if (!mountedRef.current) {
      return;
    }

    if (copyToastTimerRef.current !== null) {
      window.clearTimeout(copyToastTimerRef.current);
    }

    setCopyToast(message);

    copyToastTimerRef.current = window.setTimeout(() => {
      copyToastTimerRef.current = null;

      if (mountedRef.current) {
        setCopyToast("");
      }
    }, copyToastDurationMs);
  }, []);

  const clearClipboardTimer = useCallback(() => {
    if (clipboardTimerRef.current !== null) {
      window.clearTimeout(clipboardTimerRef.current);
      clipboardTimerRef.current = null;
    }
  }, []);

  const clearInactivityTimer = useCallback(() => {
    if (inactivityTimerRef.current !== null) {
      window.clearTimeout(inactivityTimerRef.current);
      inactivityTimerRef.current = null;
    }
  }, []);

  const isCurrentClipboardOperation = useCallback((operationId: number) => {
    return mountedRef.current && clipboardOperationRef.current === operationId;
  }, []);

  const enqueueClipboardWrite = useCallback(
    (operation: () => Promise<void>) => {
      const queuedOperation = clipboardWriteQueueRef.current
        .catch(() => undefined)
        .then(operation);

      clipboardWriteQueueRef.current = queuedOperation.catch(() => undefined);

      return queuedOperation;
    },
    [],
  );

  const triggerPrivacyReset = useCallback((message?: string) => {
    if (!mountedRef.current) {
      return;
    }

    setResetVersion((current) => current + 1);

    if (message) {
      setAnnouncement(message);
    }
  }, []);

  const scheduleInactivityTimer = useCallback(() => {
    clearInactivityTimer();

    if (status !== "authenticated" || document.visibilityState === "hidden") {
      return;
    }

    inactivityTimerRef.current = window.setTimeout(() => {
      inactivityTimerRef.current = null;
      triggerPrivacyReset(inactivityResetMessage);
    }, inactivityDelayMs);
  }, [clearInactivityTimer, inactivityDelayMs, status, triggerPrivacyReset]);

  const copyValue = useCallback(
    async ({ label, value }: CopyValueInput) => {
      clearClipboardTimer();

      const operationId = clipboardOperationRef.current + 1;
      clipboardOperationRef.current = operationId;

      const clipboard = navigator.clipboard;

      if (!clipboard?.writeText) {
        announce("Clipboard access is unavailable.");
        return false;
      }

      try {
        await enqueueClipboardWrite(async () => {
          if (!isCurrentClipboardOperation(operationId)) {
            return;
          }

          await clipboard.writeText(value);
        });
      } catch {
        if (isCurrentClipboardOperation(operationId)) {
          announce("The value could not be copied.");
        }

        return false;
      }

      if (!isCurrentClipboardOperation(operationId)) {
        return false;
      }

      showCopyToast(`${label} copied to clipboard.`);

      if (!clipboard.readText) {
        announce(
          `${label} copied. Automatic clearing is unavailable in this browser.`,
        );
        return true;
      }

      announce(
        `${label} copied. VaultForge will attempt to clear it in 30 seconds.`,
      );

      clipboardTimerRef.current = window.setTimeout(() => {
        if (!isCurrentClipboardOperation(operationId)) {
          return;
        }

        clipboardTimerRef.current = null;

        void (async () => {
          try {
            const currentClipboardText = await clipboard.readText();

            if (!isCurrentClipboardOperation(operationId)) {
              return;
            }

            if (currentClipboardText !== value) {
              announce(
                `VaultForge left ${label} unchanged because the clipboard changed.`,
              );
              return;
            }

            await enqueueClipboardWrite(async () => {
              if (!isCurrentClipboardOperation(operationId)) {
                return;
              }

              await clipboard.writeText("");
            });

            if (!isCurrentClipboardOperation(operationId)) {
              return;
            }

            announce(`${label} was cleared from the clipboard.`);
          } catch {
            if (isCurrentClipboardOperation(operationId)) {
              announce(
                `${label} was copied, but VaultForge could not clear it automatically.`,
              );
            }
          }
        })();
      }, clipboardClearDelayMs);

      return true;
    },
    [
      announce,
      clearClipboardTimer,
      clipboardClearDelayMs,
      enqueueClipboardWrite,
      isCurrentClipboardOperation,
      showCopyToast,
    ],
  );

  useEffect(() => {
    mountedRef.current = true;

    return () => {
      mountedRef.current = false;
      clipboardOperationRef.current += 1;

      clearClipboardTimer();
      clearInactivityTimer();

      if (copyToastTimerRef.current !== null) {
        window.clearTimeout(copyToastTimerRef.current);
        copyToastTimerRef.current = null;
      }
    };
  }, [clearClipboardTimer, clearInactivityTimer]);

  useEffect(() => {
    if (status !== "authenticated") {
      clearInactivityTimer();

      if (previousStatusRef.current === "authenticated") {
        triggerPrivacyReset();
      }
    } else {
      scheduleInactivityTimer();
    }

    previousStatusRef.current = status;
  }, [
    clearInactivityTimer,
    scheduleInactivityTimer,
    status,
    triggerPrivacyReset,
  ]);

  useEffect(() => {
    if (status !== "authenticated") {
      return;
    }

    const handleActivity = () => {
      scheduleInactivityTimer();
    };

    const handleVisibilityChange = () => {
      if (document.visibilityState === "hidden") {
        clearInactivityTimer();
        triggerPrivacyReset(hiddenTabResetMessage);
        return;
      }

      scheduleInactivityTimer();
    };

    document.addEventListener("pointerdown", handleActivity);
    document.addEventListener("keydown", handleActivity);
    document.addEventListener("focusin", handleActivity);
    document.addEventListener("visibilitychange", handleVisibilityChange);

    return () => {
      document.removeEventListener("pointerdown", handleActivity);
      document.removeEventListener("keydown", handleActivity);
      document.removeEventListener("focusin", handleActivity);
      document.removeEventListener("visibilitychange", handleVisibilityChange);
    };
  }, [
    clearInactivityTimer,
    scheduleInactivityTimer,
    status,
    triggerPrivacyReset,
  ]);

  const contextValue = useMemo<PrivacyContextValue>(
    () => ({
      resetVersion,
      copyValue,
    }),
    [copyValue, resetVersion],
  );

  return (
    <PrivacyContext.Provider value={contextValue}>
      {children}

      {copyToast ? (
        <div className="copy-toast" aria-hidden="true">
          {copyToast}
        </div>
      ) : null}

      <span className="visually-hidden" aria-live="polite">
        {announcement}
      </span>
    </PrivacyContext.Provider>
  );
}
