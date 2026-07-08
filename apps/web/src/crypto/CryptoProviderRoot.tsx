import type { ReactNode } from "react";
import { useEffect, useMemo, useState } from "react";

import type { CryptoRuntimeStatus } from "./CryptoContext";
import { CryptoContext } from "./CryptoContext";
import type { CryptoProvider } from "./CryptoProvider";
import { loadCryptoWasm } from "./loadCryptoWasm";
import { createWasmCryptoProvider } from "./wasmCryptoProvider";

interface CryptoProviderRootProps {
  children: ReactNode;
  provider?: CryptoProvider;
}

const defaultCryptoProvider = createWasmCryptoProvider(loadCryptoWasm);

export function CryptoProviderRoot({
  children,
  provider = defaultCryptoProvider,
}: CryptoProviderRootProps) {
  const [status, setStatus] = useState<CryptoRuntimeStatus>("loading");
  const [error, setError] = useState<unknown>(null);

  useEffect(() => {
    let active = true;

    void provider.initialize().then(
      () => {
        if (!active) {
          return;
        }

        setStatus("ready");
      },
      (initializeError: unknown) => {
        if (!active) {
          return;
        }

        setError(initializeError);
        setStatus("error");
      },
    );

    return () => {
      active = false;
    };
  }, [provider]);

  const contextValue = useMemo(
    () => ({
      provider,
      status,
      error,
    }),
    [error, provider, status],
  );

  return (
    <CryptoContext.Provider value={contextValue}>
      {children}
    </CryptoContext.Provider>
  );
}
