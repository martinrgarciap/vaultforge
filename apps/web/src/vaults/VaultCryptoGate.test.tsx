import type { ReactNode } from "react";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { Account, ApiRequestOptions } from "../api/types";
import { AuthContext } from "../auth/AuthContext";
import type { AuthContextValue } from "../auth/types";
import type { CryptoProvider } from "../crypto/CryptoProvider";
import { CryptoContext } from "../crypto/CryptoContext";
import {
  CRYPTO_ENVELOPE_VERSION,
  type ItemCryptoEnvelope,
  type WrappedKeyEnvelope,
} from "../crypto/cryptoTypes";
import { bytesToBase64 } from "../crypto/encoding";
import type { Vault } from "./contracts";
import { currentVaultCryptoVersion, currentVaultKdfVersion } from "./contracts";
import { VaultCryptoGate } from "./VaultCryptoGate";
import { VaultUnlockProvider } from "./VaultUnlockProvider";

const testAccount: Account = {
  id: "account-1",
  email: "test@example.com",
  status: "active",
  createdAt: "2026-07-08T00:00:00Z",
  updatedAt: "2026-07-08T00:00:00Z",
};

const baseVault: Vault = {
  id: "vault-123",
  name: "Development",
  createdAt: "2026-07-08T12:00:00Z",
  updatedAt: "2026-07-08T12:00:00Z",
};

const saltBytes = Uint8Array.from({ length: 32 }, (_, index) => index + 1);
const wrappedKeyBytes = Uint8Array.from(
  { length: 60 },
  (_, index) => index + 2,
);
const vaultKeyBytes = Uint8Array.from({ length: 32 }, (_, index) => index + 3);

const initializedVault: Vault = {
  ...baseVault,
  cryptoVersion: currentVaultCryptoVersion,
  kdfVersion: currentVaultKdfVersion,
  salt: bytesToBase64(saltBytes),
  wrappedKey: bytesToBase64(wrappedKeyBytes),
};

type RequestImplementation = (
  path: string,
  options?: ApiRequestOptions,
) => Promise<unknown>;

function createAuthValue(
  requestImplementation: RequestImplementation,
): AuthContextValue {
  return {
    status: "authenticated",
    account: testAccount,
    register: async () => {
      throw new Error("Unexpected registration call.");
    },
    login: async () => {
      throw new Error("Unexpected login call.");
    },
    logout: async () => undefined,
    request: requestImplementation as AuthContextValue["request"],
  };
}

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
    wrappedKey: new Uint8Array(wrappedKeyBytes),
  };
}

function createTestCryptoProvider(
  overrides: Partial<CryptoProvider> = {},
): CryptoProvider {
  return {
    initialize: vi.fn(async () => undefined),
    generateVaultKey: vi.fn(async () => new Uint8Array(vaultKeyBytes)),
    deriveKey: vi.fn(async () => new Uint8Array(32)),
    encryptItem: vi.fn(
      async (): Promise<ItemCryptoEnvelope> => createTestItemEnvelope(),
    ),
    decryptItem: vi.fn(async () => new Uint8Array()),
    wrapKey: vi.fn(
      async (): Promise<WrappedKeyEnvelope> => createTestWrappedKeyEnvelope(),
    ),
    unwrapKey: vi.fn(async () => new Uint8Array(vaultKeyBytes)),
    ...overrides,
  };
}

function Providers({
  children,
  provider,
  requestImplementation,
}: {
  children: ReactNode;
  provider: CryptoProvider;
  requestImplementation: RequestImplementation;
}) {
  return (
    <AuthContext.Provider value={createAuthValue(requestImplementation)}>
      <CryptoContext.Provider
        value={{ provider, status: "ready", error: null }}
      >
        <VaultUnlockProvider>{children}</VaultUnlockProvider>
      </CryptoContext.Provider>
    </AuthContext.Provider>
  );
}

function renderGate({
  vault,
  provider = createTestCryptoProvider(),
  requestImplementation = async () => ({
    vault: initializedVault,
  }),
  onVaultUpdated = vi.fn(),
}: {
  vault: Vault;
  provider?: CryptoProvider;
  requestImplementation?: RequestImplementation;
  onVaultUpdated?: (vault: Vault) => void;
}) {
  render(
    <Providers
      provider={provider}
      requestImplementation={requestImplementation}
    >
      <VaultCryptoGate vault={vault} onVaultUpdated={onVaultUpdated}>
        {(vaultKey) => (
          <section aria-label="Unlocked workspace">
            <p>Unlocked workspace</p>
            <p data-testid="first-key-byte">{vaultKey[0]}</p>
          </section>
        )}
      </VaultCryptoGate>
    </Providers>,
  );

  return {
    provider,
    onVaultUpdated,
  };
}

describe("VaultCryptoGate", () => {
  it("sets up crypto metadata for a new vault and unlocks it", async () => {
    const requestMock = vi.fn(async () => ({
      vault: initializedVault,
    }));
    const onVaultUpdated = vi.fn();

    renderGate({
      vault: baseVault,
      requestImplementation: requestMock,
      onVaultUpdated,
    });

    expect(
      screen.getByRole("heading", {
        name: "Set Up Vault Encryption",
      }),
    ).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Vault passphrase"), {
      target: {
        value: "synthetic passphrase",
      },
    });
    fireEvent.click(
      screen.getByRole("button", {
        name: "Set Up Vault Encryption",
      }),
    );

    expect(await screen.findByText("Unlocked workspace")).toBeInTheDocument();
    expect(screen.getByTestId("first-key-byte")).toHaveTextContent("3");

    expect(requestMock).toHaveBeenCalledWith(
      "/v1/vaults/vault-123/crypto",
      expect.objectContaining({
        method: "PUT",
        json: expect.objectContaining({
          cryptoVersion: currentVaultCryptoVersion,
          kdfVersion: currentVaultKdfVersion,
          wrappedKey: bytesToBase64(wrappedKeyBytes),
        }),
      }),
    );
    expect(onVaultUpdated).toHaveBeenCalledWith(initializedVault);
  });

  it("unlocks an initialized vault without reinitializing metadata", async () => {
    const requestMock = vi.fn(async () => ({
      vault: initializedVault,
    }));
    const provider = createTestCryptoProvider();

    renderGate({
      vault: initializedVault,
      provider,
      requestImplementation: requestMock,
    });

    expect(
      screen.getByRole("heading", {
        name: "Unlock Vault",
      }),
    ).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("Vault passphrase"), {
      target: {
        value: "synthetic passphrase",
      },
    });
    fireEvent.click(
      screen.getByRole("button", {
        name: "Unlock Vault",
      }),
    );

    expect(await screen.findByText("Unlocked workspace")).toBeInTheDocument();
    expect(provider.unwrapKey).toHaveBeenCalledTimes(1);
    expect(requestMock).not.toHaveBeenCalled();
  });

  it("requires a passphrase before setup or unlock", () => {
    renderGate({
      vault: baseVault,
    });

    fireEvent.click(
      screen.getByRole("button", {
        name: "Set Up Vault Encryption",
      }),
    );

    expect(
      screen.getByText("Vault passphrase is required."),
    ).toBeInTheDocument();
  });

  it("requires a stronger passphrase before setting up vault encryption", () => {
    const requestMock = vi.fn(async () => ({
      vault: initializedVault,
    }));

    renderGate({
      vault: baseVault,
      requestImplementation: requestMock,
    });

    fireEvent.change(screen.getByLabelText("Vault passphrase"), {
      target: {
        value: "short",
      },
    });
    fireEvent.click(
      screen.getByRole("button", {
        name: "Set Up Vault Encryption",
      }),
    );

    expect(
      screen.getByText(
        "Vault passphrase must contain between 8 and 64 characters.",
      ),
    ).toBeInTheDocument();
    expect(requestMock).not.toHaveBeenCalled();
  });

  it("allows short existing passphrases when unlocking initialized vaults", async () => {
    const provider = createTestCryptoProvider();

    renderGate({
      vault: initializedVault,
      provider,
    });

    fireEvent.change(screen.getByLabelText("Vault passphrase"), {
      target: {
        value: "short",
      },
    });
    fireEvent.click(
      screen.getByRole("button", {
        name: "Unlock Vault",
      }),
    );

    expect(await screen.findByText("Unlocked workspace")).toBeInTheDocument();
    expect(provider.unwrapKey).toHaveBeenCalledTimes(1);
  });

  it("locks an unlocked vault and returns to the unlock form", async () => {
    renderGate({
      vault: initializedVault,
    });

    fireEvent.change(screen.getByLabelText("Vault passphrase"), {
      target: {
        value: "synthetic passphrase",
      },
    });
    fireEvent.click(
      screen.getByRole("button", {
        name: "Unlock Vault",
      }),
    );

    await screen.findByText("Unlocked workspace");

    fireEvent.click(
      screen.getByRole("button", {
        name: "Lock Vault",
      }),
    );

    await waitFor(() => {
      expect(
        screen.getByRole("heading", {
          name: "Unlock Vault",
        }),
      ).toBeInTheDocument();
    });

    expect(screen.queryByText("Unlocked workspace")).not.toBeInTheDocument();
  });

  it("shows crypto runtime errors before allowing unlock", () => {
    const provider = createTestCryptoProvider();

    render(
      <AuthContext.Provider value={createAuthValue(async () => undefined)}>
        <CryptoContext.Provider
          value={{
            provider,
            status: "error",
            error: new Error("WASM failed to load"),
          }}
        >
          <VaultUnlockProvider>
            <VaultCryptoGate vault={initializedVault} onVaultUpdated={vi.fn()}>
              {() => <p>Unlocked workspace</p>}
            </VaultCryptoGate>
          </VaultUnlockProvider>
        </CryptoContext.Provider>
      </AuthContext.Provider>,
    );

    expect(screen.getByRole("alert")).toHaveTextContent(
      "The request could not be completed. Please try again.",
    );
    expect(screen.queryByLabelText("Vault passphrase")).not.toBeInTheDocument();
  });
});
