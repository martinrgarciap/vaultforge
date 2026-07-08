import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import type { Account } from "../api/types";
import { AuthContext } from "../auth/AuthContext";
import type { AuthContextValue, AuthStatus } from "../auth/types";
import { CryptoContext } from "../crypto/CryptoContext";
import type { CryptoProvider } from "../crypto/CryptoProvider";
import {
  CRYPTO_ENVELOPE_VERSION,
  type ItemCryptoEnvelope,
  type WrappedKeyEnvelope,
} from "../crypto/cryptoTypes";
import { useVaultUnlock } from "./useVaultUnlock";
import { VaultUnlockProvider } from "./VaultUnlockProvider";

const defaultTestVaultInactivityDelayMs = 30 * 60 * 1000;
const fastVaultInactivityDelayMs = 100;

const testAccount: Account = {
  id: "account-1",
  email: "test@example.com",
  status: "active",
  createdAt: "2026-07-08T00:00:00Z",
  updatedAt: "2026-07-08T00:00:00Z",
};

function createAuthValue(status: AuthStatus): AuthContextValue {
  return {
    status,
    account: status === "authenticated" ? testAccount : null,
    register: async () => testAccount,
    login: async () => testAccount,
    logout: async () => undefined,
    request: async <T,>() => undefined as T,
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
    wrappedKey: new Uint8Array(28),
  };
}

function createTestCryptoProvider(): CryptoProvider {
  return {
    initialize: vi.fn(async () => undefined),
    generateVaultKey: vi.fn(async () =>
      Uint8Array.from({ length: 32 }, (_, index) => index + 1),
    ),
    deriveKey: vi.fn(async () => new Uint8Array(32)),
    encryptItem: vi.fn(
      async (): Promise<ItemCryptoEnvelope> => createTestItemEnvelope(),
    ),
    decryptItem: vi.fn(async () => new Uint8Array()),
    wrapKey: vi.fn(
      async (): Promise<WrappedKeyEnvelope> => createTestWrappedKeyEnvelope(),
    ),
    unwrapKey: vi.fn(async () => new Uint8Array(32)),
  };
}

function Providers({
  authStatus,
  children,
  provider,
  inactivityDelayMs = defaultTestVaultInactivityDelayMs,
}: {
  authStatus: AuthStatus;
  children: ReactNode;
  provider: CryptoProvider;
  inactivityDelayMs?: number;
}) {
  return (
    <AuthContext.Provider value={createAuthValue(authStatus)}>
      <CryptoContext.Provider
        value={{ provider, status: "ready", error: null }}
      >
        <VaultUnlockProvider inactivityDelayMs={inactivityDelayMs}>
          {children}
        </VaultUnlockProvider>
      </CryptoContext.Provider>
    </AuthContext.Provider>
  );
}

function VaultUnlockControls() {
  const {
    createUnlockedVaultSession,
    getVaultKey,
    isVaultUnlocked,
    lockVault,
    unlockVaultWithKey,
    unlockedVaultIds,
  } = useVaultUnlock();

  const status = isVaultUnlocked("vault-1") ? "unlocked" : "locked";
  const key = getVaultKey("vault-1");

  return (
    <>
      <p data-testid="unlock-status">{status}</p>
      <p data-testid="unlocked-count">{unlockedVaultIds.length}</p>
      <p data-testid="first-key-byte">{key ? key[0] : "none"}</p>

      <button
        type="button"
        onClick={() => {
          void createUnlockedVaultSession("vault-1");
        }}
      >
        Create session
      </button>

      <button
        type="button"
        onClick={() => {
          const suppliedKey = Uint8Array.from(
            { length: 32 },
            (_, index) => index + 11,
          );

          unlockVaultWithKey("vault-1", suppliedKey);
          suppliedKey.fill(200);
        }}
      >
        Unlock with supplied key
      </button>

      <button
        type="button"
        onClick={() => {
          const keyCopy = getVaultKey("vault-1");

          if (keyCopy) {
            keyCopy[0] = 255;
          }
        }}
      >
        Mutate returned key
      </button>

      <button
        type="button"
        onClick={() => {
          lockVault("vault-1");
        }}
      >
        Lock vault
      </button>
    </>
  );
}

function setVisibilityState(value: DocumentVisibilityState) {
  Object.defineProperty(document, "visibilityState", {
    configurable: true,
    value,
  });
}

function waitForMilliseconds(delayMs: number): Promise<void> {
  return new Promise((resolve) => {
    window.setTimeout(resolve, delayMs);
  });
}

describe("VaultUnlockProvider", () => {
  beforeEach(() => {
    setVisibilityState("visible");
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("creates an unlocked in-memory vault session", async () => {
    const provider = createTestCryptoProvider();

    render(
      <Providers authStatus="authenticated" provider={provider}>
        <VaultUnlockControls />
      </Providers>,
    );

    expect(screen.getByTestId("unlock-status")).toHaveTextContent(/^locked$/);

    fireEvent.click(screen.getByRole("button", { name: "Create session" }));

    await waitFor(() => {
      expect(screen.getByTestId("unlock-status")).toHaveTextContent(
        /^unlocked$/,
      );
    });

    expect(screen.getByTestId("unlocked-count")).toHaveTextContent("1");
    expect(screen.getByTestId("first-key-byte")).toHaveTextContent("1");
    expect(provider.generateVaultKey).toHaveBeenCalledTimes(1);
  });

  it("stores an independent copy of supplied vault keys", async () => {
    const provider = createTestCryptoProvider();

    render(
      <Providers authStatus="authenticated" provider={provider}>
        <VaultUnlockControls />
      </Providers>,
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Unlock with supplied key" }),
    );

    await waitFor(() => {
      expect(screen.getByTestId("unlock-status")).toHaveTextContent(
        /^unlocked$/,
      );
    });

    expect(screen.getByTestId("first-key-byte")).toHaveTextContent("11");
  });

  it("returns independent key copies to callers", async () => {
    const provider = createTestCryptoProvider();

    render(
      <Providers authStatus="authenticated" provider={provider}>
        <VaultUnlockControls />
      </Providers>,
    );

    fireEvent.click(
      screen.getByRole("button", { name: "Unlock with supplied key" }),
    );

    await waitFor(() => {
      expect(screen.getByTestId("first-key-byte")).toHaveTextContent("11");
    });

    fireEvent.click(
      screen.getByRole("button", { name: "Mutate returned key" }),
    );

    expect(screen.getByTestId("first-key-byte")).toHaveTextContent("11");
  });

  it("locks a vault and removes the in-memory key", async () => {
    const provider = createTestCryptoProvider();

    render(
      <Providers authStatus="authenticated" provider={provider}>
        <VaultUnlockControls />
      </Providers>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Create session" }));

    await waitFor(() => {
      expect(screen.getByTestId("unlock-status")).toHaveTextContent(
        /^unlocked$/,
      );
    });

    fireEvent.click(screen.getByRole("button", { name: "Lock vault" }));

    expect(screen.getByTestId("unlock-status")).toHaveTextContent(/^locked$/);
    expect(screen.getByTestId("first-key-byte")).toHaveTextContent("none");
  });

  it("locks all vaults when authentication is lost", async () => {
    const provider = createTestCryptoProvider();

    const { rerender } = render(
      <Providers authStatus="authenticated" provider={provider}>
        <VaultUnlockControls />
      </Providers>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Create session" }));

    await waitFor(() => {
      expect(screen.getByTestId("unlock-status")).toHaveTextContent(
        /^unlocked$/,
      );
    });

    rerender(
      <Providers authStatus="unauthenticated" provider={provider}>
        <VaultUnlockControls />
      </Providers>,
    );

    expect(screen.getByTestId("unlock-status")).toHaveTextContent(/^locked$/);
    expect(screen.getByTestId("unlocked-count")).toHaveTextContent("0");
  });

  it("locks all vaults after inactivity", async () => {
    const provider = createTestCryptoProvider();

    render(
      <Providers
        authStatus="authenticated"
        provider={provider}
        inactivityDelayMs={fastVaultInactivityDelayMs}
      >
        <VaultUnlockControls />
      </Providers>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Create session" }));

    await waitFor(() => {
      expect(screen.getByTestId("unlock-status")).toHaveTextContent(
        /^unlocked$/,
      );
    });

    await waitFor(() => {
      expect(screen.getByTestId("unlock-status")).toHaveTextContent(/^locked$/);
      expect(screen.getByTestId("unlocked-count")).toHaveTextContent("0");
      expect(screen.getByTestId("first-key-byte")).toHaveTextContent("none");
    });
  });

  it("restarts the vault auto-lock timer after user activity", async () => {
    const provider = createTestCryptoProvider();

    render(
      <Providers
        authStatus="authenticated"
        provider={provider}
        inactivityDelayMs={fastVaultInactivityDelayMs}
      >
        <VaultUnlockControls />
      </Providers>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Create session" }));

    await waitFor(() => {
      expect(screen.getByTestId("unlock-status")).toHaveTextContent(
        /^unlocked$/,
      );
    });

    await waitForMilliseconds(fastVaultInactivityDelayMs / 2);

    fireEvent.keyDown(document);

    await waitForMilliseconds(fastVaultInactivityDelayMs / 2 + 10);

    expect(screen.getByTestId("unlock-status")).toHaveTextContent(/^unlocked$/);

    await waitFor(() => {
      expect(screen.getByTestId("unlock-status")).toHaveTextContent(/^locked$/);
    });
  });

  it("locks all vaults when the document becomes hidden", async () => {
    const provider = createTestCryptoProvider();

    render(
      <Providers
        authStatus="authenticated"
        provider={provider}
        inactivityDelayMs={fastVaultInactivityDelayMs}
      >
        <VaultUnlockControls />
      </Providers>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Create session" }));

    await waitFor(() => {
      expect(screen.getByTestId("unlock-status")).toHaveTextContent(
        /^unlocked$/,
      );
    });

    setVisibilityState("hidden");
    fireEvent(document, new Event("visibilitychange"));

    expect(screen.getByTestId("unlock-status")).toHaveTextContent(/^locked$/);
    expect(screen.getByTestId("unlocked-count")).toHaveTextContent("0");
  });
});
