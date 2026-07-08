import { createContext } from "react";

import type { CryptoProvider } from "./CryptoProvider";

export type CryptoRuntimeStatus = "loading" | "ready" | "error";

export interface CryptoContextValue {
  provider: CryptoProvider;
  status: CryptoRuntimeStatus;
  error: unknown;
}

export const CryptoContext = createContext<CryptoContextValue | null>(null);
