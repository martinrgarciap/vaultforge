import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { CryptoProvider } from "./CryptoProvider";
import { CryptoProviderRoot } from "./CryptoProviderRoot";
import {
  CRYPTO_ENVELOPE_VERSION,
  type ItemCryptoEnvelope,
  type WrappedKeyEnvelope,
} from "./cryptoTypes";
import { useCrypto } from "./useCrypto";

function createTestItemEnvelope(): ItemCryptoEnvelope {
  return {
    version: CRYPTO_ENVELOPE_VERSION,
    algorithm: "AES-256-GCM",
    blob: new Uint8Array(28),
  };
}

function createTestWrappedKeyEnvelope(): WrappedKeyEnvelope {
  return {
    version: CRYPTO_ENVELOPE_VERSION,
    algorithm: "AES-256-GCM",
    wrappedKey: new Uint8Array(28),
  };
}

function createTestCryptoProvider(
  overrides: Partial<CryptoProvider> = {},
): CryptoProvider {
  return {
    initialize: vi.fn(async () => undefined),
    generateVaultKey: vi.fn(async () => new Uint8Array(32)),
    deriveKey: vi.fn(async () => new Uint8Array(32)),
    encryptItem: vi.fn(
      async (): Promise<ItemCryptoEnvelope> => createTestItemEnvelope(),
    ),
    decryptItem: vi.fn(async () => new Uint8Array()),
    wrapKey: vi.fn(
      async (): Promise<WrappedKeyEnvelope> => createTestWrappedKeyEnvelope(),
    ),
    unwrapKey: vi.fn(async () => new Uint8Array(32)),
    ...overrides,
  };
}

function CryptoStatusProbe() {
  const { status, error } = useCrypto();

  return (
    <>
      <p data-testid="crypto-status">{status}</p>
      <p data-testid="crypto-error">
        {error instanceof Error ? error.message : ""}
      </p>
    </>
  );
}

describe("CryptoProviderRoot", () => {
  it("initializes the crypto provider and exposes ready status", async () => {
    const provider = createTestCryptoProvider();

    render(
      <CryptoProviderRoot provider={provider}>
        <CryptoStatusProbe />
      </CryptoProviderRoot>,
    );

    expect(screen.getByTestId("crypto-status")).toHaveTextContent("loading");
    expect(await screen.findByText("ready")).toBeInTheDocument();
    expect(provider.initialize).toHaveBeenCalledTimes(1);
  });

  it("exposes initialization failures", async () => {
    const provider = createTestCryptoProvider({
      initialize: vi.fn(async () => {
        throw new Error("WASM failed to load");
      }),
    });

    render(
      <CryptoProviderRoot provider={provider}>
        <CryptoStatusProbe />
      </CryptoProviderRoot>,
    );

    expect(await screen.findByText("error")).toBeInTheDocument();
    expect(screen.getByTestId("crypto-error")).toHaveTextContent(
      "WASM failed to load",
    );
  });
});
