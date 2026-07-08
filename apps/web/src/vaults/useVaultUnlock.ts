import { useContext } from "react";

import type { VaultUnlockContextValue } from "./VaultUnlockContext";
import { VaultUnlockContext } from "./VaultUnlockContext";

export function useVaultUnlock(): VaultUnlockContextValue {
  const context = useContext(VaultUnlockContext);

  if (!context) {
    throw new Error(
      "useVaultUnlock must be used within a VaultUnlockProvider.",
    );
  }

  return context;
}
