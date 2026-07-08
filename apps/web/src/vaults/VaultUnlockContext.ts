import { createContext } from "react";

export interface VaultUnlockContextValue {
  unlockedVaultIds: string[];
  createUnlockedVaultSession: (vaultId: string) => Promise<Uint8Array>;
  getVaultKey: (vaultId: string) => Uint8Array | null;
  isVaultUnlocked: (vaultId: string) => boolean;
  lockVault: (vaultId: string) => void;
  lockAllVaults: () => void;
  unlockVaultWithKey: (vaultId: string, key: Uint8Array) => void;
}

export const VaultUnlockContext = createContext<VaultUnlockContextValue | null>(
  null,
);
