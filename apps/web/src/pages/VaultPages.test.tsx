import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import type { ReactNode } from "react";
import { MemoryRouter, Route, Routes } from "react-router";
import { describe, expect, it, vi } from "vitest";

import { ApiError } from "../api/ApiError";
import type { ApiRequestOptions } from "../api/types";
import { AuthContext } from "../auth/AuthContext";
import type { AuthContextValue, AuthStatus } from "../auth/types";
import { CryptoContext } from "../crypto/CryptoContext";
import type { CryptoProvider } from "../crypto/CryptoProvider";
import {
  CRYPTO_ENVELOPE_VERSION,
  type ItemCryptoEnvelope,
  type WrappedKeyEnvelope,
} from "../crypto/cryptoTypes";
import { VaultUnlockProvider } from "../vaults/VaultUnlockProvider";
import { VaultDetailPage } from "./VaultDetailPage";
import { VaultsPage } from "./VaultsPage";

const vault = {
  id: "vault-123",
  name: "Development",
  createdAt: "2026-06-22T12:00:00Z",
  updatedAt: "2026-06-22T12:00:00Z",
};

type RequestImplementation = (
  path: string,
  options?: ApiRequestOptions,
) => Promise<unknown>;

function createAuthValue(
  requestImplementation: RequestImplementation,
  status: AuthStatus = "authenticated",
): AuthContextValue {
  return {
    status,
    account: null,
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
    wrappedKey: new Uint8Array(60),
  };
}

function createTestCryptoProvider(): CryptoProvider {
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
  };
}

function TestCryptoProviders({
  children,
  provider = createTestCryptoProvider(),
}: {
  children: ReactNode;
  provider?: CryptoProvider;
}) {
  return (
    <CryptoContext.Provider value={{ provider, status: "ready", error: null }}>
      <VaultUnlockProvider>{children}</VaultUnlockProvider>
    </CryptoContext.Provider>
  );
}

function renderVaultList(
  requestImplementation: RequestImplementation,
  status: AuthStatus = "authenticated",
) {
  render(
    <MemoryRouter initialEntries={["/vaults"]}>
      <AuthContext.Provider
        value={createAuthValue(requestImplementation, status)}
      >
        <Routes>
          <Route path="/vaults" element={<VaultsPage />} />

          <Route path="/vaults/:vaultId" element={<h1>Vault destination</h1>} />

          <Route path="/login" element={<h1>Login destination</h1>} />
        </Routes>
      </AuthContext.Provider>
    </MemoryRouter>,
  );
}

async function unlockVault() {
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

  await screen.findByRole("button", {
    name: "Edit",
  });
}

function renderVaultDetail(requestImplementation: RequestImplementation) {
  render(
    <MemoryRouter initialEntries={["/vaults/vault-123"]}>
      <AuthContext.Provider value={createAuthValue(requestImplementation)}>
        <TestCryptoProviders>
          <Routes>
            <Route path="/vaults/:vaultId" element={<VaultDetailPage />} />

            <Route path="/vaults" element={<h1>Vault List destination</h1>} />
          </Routes>
        </TestCryptoProviders>
      </AuthContext.Provider>
    </MemoryRouter>,
  );
}

describe("VaultsPage", () => {
  it("loads a clickable Vault List", async () => {
    const requestMock = vi.fn(async () => ({
      vaults: [vault],
    }));

    renderVaultList(requestMock);

    expect(
      await screen.findByRole("heading", {
        name: "Vault List",
      }),
    ).toBeInTheDocument();

    const row = screen.getByRole("link", {
      name: "Open Development",
    });

    expect(
      screen.getByRole("columnheader", {
        name: "Last Updated",
      }),
    ).toBeInTheDocument();

    fireEvent.click(row);

    expect(
      await screen.findByRole("heading", {
        name: "Vault destination",
      }),
    ).toBeInTheDocument();

    expect(requestMock).toHaveBeenCalledWith("/v1/vaults");
  });

  it("displays the empty state", async () => {
    const requestMock = vi.fn(async () => ({
      vaults: [],
    }));

    renderVaultList(requestMock);

    expect(
      await screen.findByText("You do not have any vaults yet."),
    ).toBeInTheDocument();
  });

  it("creates a normalized vault and opens it", async () => {
    const requestMock = vi.fn(
      async (_path: string, options?: ApiRequestOptions) => {
        if (options?.method === "POST") {
          return {
            vault: {
              ...vault,
              name: "Dévelopment Vault",
            },
          };
        }

        return {
          vaults: [],
        };
      },
    );

    renderVaultList(requestMock);

    await screen.findByText("You do not have any vaults yet.");

    fireEvent.click(
      screen.getByRole("button", {
        name: "Create Vault",
      }),
    );

    const dialog = screen.getByRole("dialog", {
      name: "Create Vault",
    });

    fireEvent.change(within(dialog).getByLabelText("Vault name"), {
      target: {
        value: "  Dévelopment Vault  ",
      },
    });

    fireEvent.click(
      within(dialog).getByRole("button", {
        name: "Create Vault",
      }),
    );

    expect(
      await screen.findByRole("heading", {
        name: "Vault destination",
      }),
    ).toBeInTheDocument();

    const createCall = requestMock.mock.calls.find(
      ([, options]) => options?.method === "POST",
    );

    expect(createCall).toEqual([
      "/v1/vaults",
      {
        method: "POST",
        json: {
          name: "Dévelopment Vault",
        },
      },
    ]);
  });

  it("rejects an invalid vault name before requesting", async () => {
    const requestMock = vi.fn(async () => ({
      vaults: [],
    }));

    renderVaultList(requestMock);

    await screen.findByText("You do not have any vaults yet.");

    fireEvent.click(
      screen.getByRole("button", {
        name: "Create Vault",
      }),
    );

    const dialog = screen.getByRole("dialog", {
      name: "Create Vault",
    });

    fireEvent.change(within(dialog).getByLabelText("Vault name"), {
      target: {
        value: "   ",
      },
    });

    fireEvent.click(
      within(dialog).getByRole("button", {
        name: "Create Vault",
      }),
    );

    expect(
      within(dialog).getByText(
        "Vault name must be valid Unicode, contain no control characters, and be between 1 and 128 characters.",
      ),
    ).toBeInTheDocument();

    expect(requestMock).toHaveBeenCalledTimes(1);
  });

  it("displays safe vault-list API errors", async () => {
    const requestMock = vi.fn(async () => {
      throw new ApiError(
        503,
        "vault_unavailable",
        "Vault operations are temporarily unavailable.",
        "request-123",
      );
    });

    renderVaultList(requestMock);

    const alert = await screen.findByRole("alert");

    expect(alert).toHaveTextContent(
      "Vault operations are temporarily unavailable.",
    );
  });

  it("does not request vaults while unauthenticated", () => {
    const requestMock = vi.fn(async () => ({
      vaults: [],
    }));

    renderVaultList(requestMock, "unauthenticated");

    expect(
      screen.getByText("Sign in to view and manage your vaults."),
    ).toBeInTheDocument();

    expect(requestMock).not.toHaveBeenCalled();
  });
});

describe("VaultDetailPage", () => {
  it("loads and edits a vault through a modal", async () => {
    const requestMock = vi.fn(
      async (path: string, options?: ApiRequestOptions) => {
        if (options?.method === "PATCH") {
          return {
            vault: {
              ...vault,
              name: "Production",
              updatedAt: "2026-06-22T13:00:00Z",
            },
          };
        }

        if (path.endsWith("/crypto") && options?.method === "PUT") {
          return {
            vault,
          };
        }

        if (path.includes("/items?")) {
          return {
            items: [],
          };
        }

        return {
          vault,
        };
      },
    );

    renderVaultDetail(requestMock);

    expect(await screen.findByText("Development")).toBeInTheDocument();

    expect(
      screen.getByRole("heading", {
        name: "Vault Details",
      }),
    ).toBeInTheDocument();

    expect(
      screen.getByRole("link", {
        name: "Back to Vault List",
      }),
    ).toBeInTheDocument();

    expect(screen.queryByText("Crypto version")).not.toBeInTheDocument();

    expect(screen.queryByText("KDF version")).not.toBeInTheDocument();

    expect(screen.queryByText("vault-123")).not.toBeInTheDocument();

    await unlockVault();

    fireEvent.click(
      screen.getByRole("button", {
        name: "Edit",
      }),
    );

    const dialog = screen.getByRole("dialog", {
      name: "Edit Vault",
    });

    const nameInput = within(dialog).getByLabelText("Vault name");

    expect(nameInput).toHaveValue("Development");

    fireEvent.change(nameInput, {
      target: {
        value: "  Production  ",
      },
    });

    fireEvent.click(
      within(dialog).getByRole("button", {
        name: "Save Changes",
      }),
    );

    await waitFor(() => {
      expect(screen.getByText("Production")).toBeInTheDocument();
    });

    expect(
      screen.queryByRole("dialog", {
        name: "Edit Vault",
      }),
    ).not.toBeInTheDocument();

    expect(requestMock).toHaveBeenCalledWith("/v1/vaults/vault-123", {
      method: "PATCH",
      json: {
        name: "Production",
      },
    });
  });

  it("requires modal confirmation before deleting a vault", async () => {
    const requestMock = vi.fn(
      async (path: string, options?: ApiRequestOptions) => {
        if (options?.method === "DELETE") {
          return undefined;
        }

        if (path.endsWith("/crypto") && options?.method === "PUT") {
          return {
            vault,
          };
        }

        if (path.includes("/items?")) {
          return {
            items: [],
          };
        }

        return {
          vault,
        };
      },
    );

    renderVaultDetail(requestMock);

    await screen.findByRole("heading", {
      name: "Vault Details",
    });

    await unlockVault();

    fireEvent.click(
      screen.getByRole("button", {
        name: "Delete",
      }),
    );

    const dialog = screen.getByRole("dialog", {
      name: "Delete Vault?",
    });

    expect(
      requestMock.mock.calls.some(
        ([, options]) => options?.method === "DELETE",
      ),
    ).toBe(false);

    fireEvent.click(
      within(dialog).getByRole("button", {
        name: "Delete Vault",
      }),
    );

    expect(
      await screen.findByRole("heading", {
        name: "Vault List destination",
      }),
    ).toBeInTheDocument();

    expect(requestMock).toHaveBeenCalledWith("/v1/vaults/vault-123", {
      method: "DELETE",
    });
  });

  it("shows a non-retryable vault-not-found state", async () => {
    const requestMock = vi.fn(async () => {
      throw new ApiError(
        404,
        "vault_not_found",
        "The vault was not found.",
        "request-vault-missing",
      );
    });

    renderVaultDetail(requestMock);

    expect(
      await screen.findByRole("heading", {
        name: "Vault Not Found",
      }),
    ).toBeInTheDocument();

    expect(
      screen.getByRole("link", {
        name: "Return to Vault List",
      }),
    ).toHaveAttribute("href", "/vaults");

    expect(
      screen.queryByRole("button", {
        name: "Try again",
      }),
    ).not.toBeInTheDocument();
  });
});
