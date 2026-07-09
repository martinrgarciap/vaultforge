import type { ReactNode } from "react";
import { useCallback, useMemo, useState } from "react";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";

import { ApiError } from "../api/ApiError";
import type { Account } from "../api/types";
import { AuthContext } from "../auth/AuthContext";
import type { AuthContextValue, AuthStatus } from "../auth/types";
import type { CryptoProvider } from "../crypto/CryptoProvider";
import { CryptoContext } from "../crypto/CryptoContext";
import {
  CRYPTO_ENVELOPE_VERSION,
  type ItemCryptoEnvelope,
  type WrappedKeyEnvelope,
} from "../crypto/cryptoTypes";
import { bytesToBase64 } from "../crypto/encoding";
import {
  itemEncryptedPayloadAlgorithm,
  minimumItemEncryptedPayloadBlobBytes,
} from "../items/encryptedPayload";
import { encodeItemPlaintext } from "../items/itemEncryption";
import { PrivacyProvider } from "../privacy/PrivacyProvider";
import { VaultUnlockContext } from "../vaults/VaultUnlockContext";
import { AppRoutes } from "./AppRoutes";

const authenticatedAccount: Account = {
  id: "user-123",
  email: "developer@example.com",
  status: "active",
  createdAt: "2026-06-22T12:00:00Z",
  updatedAt: "2026-06-22T12:00:00Z",
};

function createAuthValue(
  overrides: Partial<AuthContextValue> = {},
): AuthContextValue {
  return {
    status: "unauthenticated",
    account: null,
    register: async () => {
      throw new Error("Unexpected registration call.");
    },
    login: async () => {
      throw new Error("Unexpected login call.");
    },
    logout: async () => undefined,
    request: async <T,>() => undefined as T,
    ...overrides,
  };
}

const testVaultKey = new Uint8Array(32);

function validEncryptedBlob(): Uint8Array {
  const blob = new Uint8Array(minimumItemEncryptedPayloadBlobBytes);

  for (let index = 0; index < blob.length; index += 1) {
    blob[index] = index + 1;
  }

  return blob;
}

function encryptedPayloadForPayload(payload: Record<string, unknown>) {
  const plaintext = encodeItemPlaintext(payload);
  const prefix = validEncryptedBlob();
  const blob = new Uint8Array(prefix.length + plaintext.length);

  blob.set(prefix);
  blob.set(plaintext, prefix.length);

  return {
    version: CRYPTO_ENVELOPE_VERSION,
    algorithm: itemEncryptedPayloadAlgorithm,
    blob: bytesToBase64(blob),
  };
}

function decryptTestEnvelope(envelope: ItemCryptoEnvelope): Uint8Array {
  return envelope.blob.slice(validEncryptedBlob().length);
}

function createTestCryptoProvider(): CryptoProvider {
  const itemEnvelope: ItemCryptoEnvelope = {
    version: CRYPTO_ENVELOPE_VERSION,
    algorithm: itemEncryptedPayloadAlgorithm,
    blob: validEncryptedBlob(),
  };
  const wrappedKeyEnvelope: WrappedKeyEnvelope = {
    version: CRYPTO_ENVELOPE_VERSION,
    algorithm: "AES-256-GCM",
    wrappedKey: new Uint8Array(60),
  };

  return {
    initialize: vi.fn(async () => undefined),
    generateVaultKey: vi.fn(async () => new Uint8Array(32)),
    deriveKey: vi.fn(async () => new Uint8Array(32)),
    encryptItem: vi.fn(async () => itemEnvelope),
    decryptItem: vi.fn(async (_vaultKey, envelope) =>
      decryptTestEnvelope(envelope),
    ),
    wrapKey: vi.fn(async () => wrappedKeyEnvelope),
    unwrapKey: vi.fn(async () => new Uint8Array(32)),
  };
}

function TestCryptoProviders({ children }: { children: ReactNode }) {
  return (
    <CryptoContext.Provider
      value={{
        provider: createTestCryptoProvider(),
        status: "ready",
        error: null,
      }}
    >
      <VaultUnlockContext.Provider
        value={{
          unlockedVaultIds: ["vault-123"],
          createUnlockedVaultSession: async () => new Uint8Array(testVaultKey),
          getVaultKey: () => new Uint8Array(testVaultKey),
          isVaultUnlocked: () => true,
          lockVault: vi.fn(),
          lockAllVaults: vi.fn(),
          unlockVaultWithKey: vi.fn(),
        }}
      >
        {children}
      </VaultUnlockContext.Provider>
    </CryptoContext.Provider>
  );
}

function renderRoute(
  path: string,
  authOverrides: Partial<AuthContextValue> = {},
) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <AuthContext.Provider value={createAuthValue(authOverrides)}>
        <PrivacyProvider>
          <TestCryptoProviders>
            <AppRoutes />
          </TestCryptoProviders>
        </PrivacyProvider>
      </AuthContext.Provider>
    </MemoryRouter>,
  );
}

describe("AppRoutes", () => {
  it("renders the public home page for signed-out root visits", () => {
    renderRoute("/");

    expect(
      screen.getByRole("heading", {
        name: "VaultForge",
      }),
    ).toBeInTheDocument();

    expect(
      screen.getByRole("link", {
        name: "Create demo account",
      }),
    ).toHaveAttribute("href", "/register");

    expect(
      screen.getByRole("link", {
        name: "Sign in",
      }),
    ).toHaveAttribute("href", "/login");
  });

  it("renders the public home page with a greeting for signed-in root visits", () => {
    renderRoute("/", {
      status: "authenticated",
      account: authenticatedAccount,
    });

    expect(
      screen.getByRole("heading", {
        name: "VaultForge",
      }),
    ).toBeInTheDocument();

    expect(
      screen.getByText("Hello, developer@example.com"),
    ).toBeInTheDocument();

    expect(
      screen.getByRole("link", {
        name: "Open Vaults",
      }),
    ).toHaveAttribute("href", "/vaults");
  });

  it("shows one restoration state before routing", () => {
    renderRoute("/vaults", {
      status: "restoring",
    });

    expect(
      screen.getByRole("heading", {
        name: "Checking Your Session",
      }),
    ).toBeInTheDocument();

    expect(screen.getByRole("status")).toHaveTextContent(
      "Restoring your session...",
    );
  });

  it.each([
    ["/register", "Create your account"],
    ["/login", "Sign in"],
    ["/generate", "Password Generator"],
  ])("renders signed-out public route %s", (path, heading) => {
    renderRoute(path);

    expect(
      screen.getByRole("heading", {
        name: heading,
      }),
    ).toBeInTheDocument();
  });

  it.each([
    "/vaults",
    "/sessions",
    "/vaults/vault-123",
    "/vaults/vault-123/items/item-123?state=active",
  ])("redirects signed-out protected route %s to login", async (path) => {
    renderRoute(path);

    expect(
      await screen.findByRole("heading", {
        name: "Sign in",
      }),
    ).toBeInTheDocument();

    expect(screen.getByRole("status")).toHaveTextContent(
      "Your session is not active. Sign in to continue.",
    );
  });

  it.each(["/login", "/register"])(
    "redirects signed-in public route %s to vaults",
    async (path) => {
      const requestMock = vi.fn(async () => ({
        vaults: [],
      }));

      renderRoute(path, {
        status: "authenticated",
        request: requestMock as AuthContextValue["request"],
      });

      expect(
        await screen.findByRole("heading", {
          name: "Your Vaults",
        }),
      ).toBeInTheDocument();
    },
  );

  it("renders the password generator while signed in", () => {
    renderRoute("/generate", {
      status: "authenticated",
    });

    expect(
      screen.getByRole("heading", {
        name: "Password Generator",
      }),
    ).toBeInTheDocument();

    expect(
      screen.getByRole("link", {
        name: "Password Generator",
      }),
    ).toBeInTheDocument();
  });

  it("loads the selected vault route", async () => {
    const requestMock = vi.fn(async (path: string) => {
      if (path.includes("/items?")) {
        return {
          items: [],
        };
      }

      return {
        vault: {
          id: "vault-123",
          name: "Development",
          createdAt: "2026-06-22T12:00:00Z",
          updatedAt: "2026-06-22T12:00:00Z",
        },
      };
    });

    renderRoute("/vaults/vault-123", {
      status: "authenticated",
      request: requestMock as AuthContextValue["request"],
    });

    expect(await screen.findByText("Development")).toBeInTheDocument();

    expect(
      screen.getByRole("heading", {
        name: "Vault Details",
      }),
    ).toBeInTheDocument();
  });

  it("loads the selected item route", async () => {
    const requestMock = vi.fn(async (path: string) => {
      if (path === "/v1/vaults/vault-123") {
        return {
          vault: {
            id: "vault-123",
            name: "Development Vault",
            createdAt: "2026-06-22T11:00:00Z",
            updatedAt: "2026-06-22T12:00:00Z",
          },
        };
      }

      return {
        item: {
          id: "item-123",
          type: "secure_note",
          encryptedPayload: encryptedPayloadForPayload({
            title: "Synthetic Note",
            note: "Synthetic content.",
          }),
          version: 1,
          createdAt: "2026-06-22T12:00:00Z",
          updatedAt: "2026-06-22T12:00:00Z",
        },
      };
    });

    renderRoute("/vaults/vault-123/items/item-123?state=active", {
      status: "authenticated",
      request: requestMock as AuthContextValue["request"],
    });

    expect(
      await screen.findByRole("heading", {
        name: "Synthetic Note",
      }),
    ).toBeInTheDocument();
  });

  it("renders an authentication-aware not-found page", () => {
    renderRoute("/unknown", {
      status: "authenticated",
    });

    expect(
      screen.getByRole("heading", {
        name: "Page Not Found",
      }),
    ).toBeInTheDocument();

    expect(
      screen.getByRole("link", {
        name: "Return to Vaults",
      }),
    ).toHaveAttribute("href", "/vaults");
  });

  it("removes protected content after authentication loss", async () => {
    function AuthenticationLossHarness() {
      const [status, setStatus] = useState<AuthStatus>("authenticated");

      const request = useCallback(async <T,>(): Promise<T> => {
        await Promise.resolve();

        setStatus("unauthenticated");

        throw new ApiError(
          401,
          "unauthorized",
          "A valid access token is required.",
        );
      }, []);

      const value = useMemo<AuthContextValue>(
        () => ({
          status,
          account: null,
          register: async () => {
            throw new Error("Unexpected registration call.");
          },
          login: async () => {
            throw new Error("Unexpected login call.");
          },
          logout: async () => undefined,
          request,
        }),
        [request, status],
      );

      return (
        <AuthContext.Provider value={value}>
          <PrivacyProvider>
            <TestCryptoProviders>
              <AppRoutes />
            </TestCryptoProviders>
          </PrivacyProvider>
        </AuthContext.Provider>
      );
    }

    render(
      <MemoryRouter initialEntries={["/vaults"]}>
        <AuthenticationLossHarness />
      </MemoryRouter>,
    );

    expect(
      await screen.findByRole("heading", {
        name: "Sign in",
      }),
    ).toBeInTheDocument();

    expect(
      screen.queryByRole("heading", {
        name: "Vault List",
      }),
    ).not.toBeInTheDocument();
  });

  it("renders the shared application shell", () => {
    renderRoute("/login");

    expect(
      screen.getByRole("navigation", {
        name: "Primary navigation",
      }),
    ).toBeInTheDocument();

    expect(
      screen.getByText(
        "Use synthetic data only. Vault item payloads are encrypted in the browser before they are sent to the API. Unlocked vault keys and revealed values are cleared after inactivity or when the tab is hidden.",
      ),
    ).toBeInTheDocument();
  });
});
