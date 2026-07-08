import { useContext } from "react";

import type { CryptoContextValue } from "./CryptoContext";
import { CryptoContext } from "./CryptoContext";

export function useCrypto(): CryptoContextValue {
  const context = useContext(CryptoContext);

  if (!context) {
    throw new Error("useCrypto must be used within a CryptoProviderRoot.");
  }

  return context;
}
