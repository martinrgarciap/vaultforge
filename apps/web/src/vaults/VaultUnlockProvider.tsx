import type { ReactNode } from "react";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import { useAuth } from "../auth/useAuth";
import { KEY_BYTES } from "../crypto/cryptoTypes";
import { useCrypto } from "../crypto/useCrypto";
import { VaultUnlockContext } from "./VaultUnlockContext";

interface VaultUnlockProviderProps {
  children: ReactNode;
  inactivityDelayMs?: number;
}

const defaultVaultInactivityDelayMs = 5 * 60 * 1000;

type VaultKeyStore = Map<string, Uint8Array>;

export function VaultUnlockProvider({
  children,
  inactivityDelayMs = defaultVaultInactivityDelayMs,
}: VaultUnlockProviderProps) {
  const { status: authStatus } = useAuth();
  const { provider } = useCrypto();

  const vaultKeysRef = useRef<VaultKeyStore>(new Map());
  const inactivityTimerRef = useRef<ReturnType<
    typeof window.setTimeout
  > | null>(null);
  const [unlockedVaultIds, setUnlockedVaultIds] = useState<string[]>([]);

  const publishUnlockedVaultIds = useCallback(() => {
    setUnlockedVaultIds(Array.from(vaultKeysRef.current.keys()).sort());
  }, []);

  const getVaultKey = useCallback((vaultId: string): Uint8Array | null => {
    const key = vaultKeysRef.current.get(normalizeVaultId(vaultId));

    if (!key) {
      return null;
    }

    return cloneVaultKey(key);
  }, []);

  const clearInactivityTimer = useCallback(() => {
    if (inactivityTimerRef.current !== null) {
      window.clearTimeout(inactivityTimerRef.current);
      inactivityTimerRef.current = null;
    }
  }, []);

  const lockAllVaults = useCallback(() => {
    clearInactivityTimer();

    const hadUnlockedVaults = vaultKeysRef.current.size > 0;

    for (const key of vaultKeysRef.current.values()) {
      zeroizeKey(key);
    }

    vaultKeysRef.current.clear();

    if (hadUnlockedVaults) {
      publishUnlockedVaultIds();
    }
  }, [clearInactivityTimer, publishUnlockedVaultIds]);

  const scheduleInactivityTimer = useCallback(() => {
    clearInactivityTimer();

    if (vaultKeysRef.current.size === 0) {
      return;
    }

    inactivityTimerRef.current = window.setTimeout(() => {
      inactivityTimerRef.current = null;
      lockAllVaults();
    }, inactivityDelayMs);
  }, [clearInactivityTimer, inactivityDelayMs, lockAllVaults]);

  const lockVault = useCallback(
    (vaultId: string) => {
      const normalizedVaultId = normalizeVaultId(vaultId);
      const key = vaultKeysRef.current.get(normalizedVaultId);

      if (!key) {
        return;
      }

      zeroizeKey(key);
      vaultKeysRef.current.delete(normalizedVaultId);
      publishUnlockedVaultIds();
      scheduleInactivityTimer();
    },
    [publishUnlockedVaultIds, scheduleInactivityTimer],
  );

  const unlockVaultWithKey = useCallback(
    (vaultId: string, key: Uint8Array) => {
      const normalizedVaultId = normalizeVaultId(vaultId);
      const keyCopy = cloneVaultKey(key);
      const existingKey = vaultKeysRef.current.get(normalizedVaultId);

      if (existingKey) {
        zeroizeKey(existingKey);
      }

      vaultKeysRef.current.set(normalizedVaultId, keyCopy);
      publishUnlockedVaultIds();
      scheduleInactivityTimer();
    },
    [publishUnlockedVaultIds, scheduleInactivityTimer],
  );

  const createUnlockedVaultSession = useCallback(
    async (vaultId: string): Promise<Uint8Array> => {
      const generatedKey = await provider.generateVaultKey();

      try {
        unlockVaultWithKey(vaultId, generatedKey);

        const storedKey = getVaultKey(vaultId);

        if (!storedKey) {
          throw new Error("Vault key was not stored after unlock.");
        }

        return storedKey;
      } finally {
        zeroizeKey(generatedKey);
      }
    },
    [getVaultKey, provider, unlockVaultWithKey],
  );

  const isVaultUnlocked = useCallback(
    (vaultId: string): boolean =>
      unlockedVaultIds.includes(normalizeVaultId(vaultId)),
    [unlockedVaultIds],
  );

  useEffect(() => {
    if (authStatus !== "authenticated") {
      lockAllVaults();
    }
  }, [authStatus, lockAllVaults]);

  useEffect(() => {
    if (authStatus !== "authenticated") {
      clearInactivityTimer();
      return;
    }

    scheduleInactivityTimer();

    return () => {
      clearInactivityTimer();
    };
  }, [
    authStatus,
    clearInactivityTimer,
    scheduleInactivityTimer,
    unlockedVaultIds,
  ]);

  useEffect(() => {
    if (authStatus !== "authenticated") {
      return;
    }

    const handleActivity = () => {
      scheduleInactivityTimer();
    };

    const handleVisibilityChange = () => {
      if (document.visibilityState === "hidden") {
        lockAllVaults();
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
  }, [authStatus, lockAllVaults, scheduleInactivityTimer]);

  useEffect(
    () => () => {
      lockAllVaults();
    },
    [lockAllVaults],
  );

  const contextValue = useMemo(
    () => ({
      unlockedVaultIds,
      createUnlockedVaultSession,
      getVaultKey,
      isVaultUnlocked,
      lockVault,
      lockAllVaults,
      unlockVaultWithKey,
    }),
    [
      createUnlockedVaultSession,
      getVaultKey,
      isVaultUnlocked,
      lockAllVaults,
      lockVault,
      unlockVaultWithKey,
      unlockedVaultIds,
    ],
  );

  return (
    <VaultUnlockContext.Provider value={contextValue}>
      {children}
    </VaultUnlockContext.Provider>
  );
}

function normalizeVaultId(vaultId: string): string {
  if (vaultId.length === 0 || vaultId.trim() !== vaultId) {
    throw new TypeError("Vault ID is required.");
  }

  return vaultId;
}

function cloneVaultKey(key: Uint8Array): Uint8Array {
  if (!(key instanceof Uint8Array) || key.length !== KEY_BYTES) {
    throw new TypeError(`Vault key must be ${KEY_BYTES} bytes.`);
  }

  return new Uint8Array(key);
}

function zeroizeKey(key: Uint8Array): void {
  key.fill(0);
}
