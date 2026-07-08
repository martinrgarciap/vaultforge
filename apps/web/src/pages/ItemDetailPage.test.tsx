import type { ReactNode } from "react";
import {
  fireEvent,
  render,
  screen,
  waitFor,
  within,
} from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "../api/ApiError";
import type { ApiRequestOptions } from "../api/types";
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
import {
  itemEncryptedPayloadAlgorithm,
  minimumItemEncryptedPayloadBlobBytes,
} from "../items/encryptedPayload";
import { encodeItemPlaintext } from "../items/itemEncryption";
import { PrivacyProvider } from "../privacy/PrivacyProvider";
import { VaultUnlockContext } from "../vaults/VaultUnlockContext";
import { ItemDetailPage } from "./ItemDetailPage";

const vault = {
  id: "vault-123",
  name: "Development Vault",
  createdAt: "2026-06-22T11:00:00Z",
  updatedAt: "2026-06-22T12:00:00Z",
};

const activeItem = {
  id: "item-123",
  type: "login",
  payload: {
    title: "Test Login",
    username: "demo-user",
    password: "synthetic-password",
    website: "https://example.test",
  },
  version: 2,
  createdAt: "2026-06-22T12:00:00Z",
  updatedAt: "2026-06-22T12:30:00Z",
} as const;

const deletedItem = {
  ...activeItem,
  version: 3,
  deletedAt: "2026-06-22T13:00:00Z",
};

type RequestImplementation = (
  path: string,
  options?: ApiRequestOptions,
) => Promise<unknown>;

const testVaultKey = Uint8Array.from({ length: 32 }, (_, index) => index + 1);

const writeTextMock = vi.fn();

beforeEach(() => {
  writeTextMock.mockReset();
  writeTextMock.mockResolvedValue(undefined);

  Object.defineProperty(navigator, "clipboard", {
    configurable: true,
    value: {
      writeText: writeTextMock,
    },
  });
});

function validEncryptedBlob(): Uint8Array {
  return Uint8Array.from(
    { length: minimumItemEncryptedPayloadBlobBytes + 4 },
    (_, index) => index + 2,
  );
}

function encryptedPayloadForPayload(payload: unknown) {
  const plaintext = encodeItemPlaintext(payload as Record<string, unknown>);
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

function encryptedItemResource<T extends { payload: unknown }>(item: T) {
  const { payload, ...resource } = item;

  return {
    ...resource,
    encryptedPayload: encryptedPayloadForPayload(payload),
  };
}

function decryptTestEnvelope(envelope: ItemCryptoEnvelope): Uint8Array {
  return envelope.blob.slice(validEncryptedBlob().length);
}

function createTestItemEnvelope(): ItemCryptoEnvelope {
  return {
    version: CRYPTO_ENVELOPE_VERSION,
    algorithm: itemEncryptedPayloadAlgorithm,
    blob: validEncryptedBlob(),
  };
}

function createTestWrappedKeyEnvelope(): WrappedKeyEnvelope {
  return {
    version: CRYPTO_ENVELOPE_VERSION,
    algorithm: itemEncryptedPayloadAlgorithm,
    wrappedKey: validEncryptedBlob(),
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
    decryptItem: vi.fn(async (_vaultKey, envelope) =>
      decryptTestEnvelope(envelope),
    ),
    wrapKey: vi.fn(
      async (): Promise<WrappedKeyEnvelope> => createTestWrappedKeyEnvelope(),
    ),
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

function renderItemPage(
  requestImplementation: RequestImplementation,
  initialPath = "/vaults/vault-123/items/item-123?state=active",
) {
  const authValue: AuthContextValue = {
    status: "authenticated",
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

  render(
    <MemoryRouter initialEntries={[initialPath]}>
      <AuthContext.Provider value={authValue}>
        <PrivacyProvider>
          <TestCryptoProviders>
            <Routes>
              <Route
                path="/vaults/:vaultId/items/:itemId"
                element={<ItemDetailPage />}
              />

              <Route
                path="/vaults/:vaultId"
                element={<h1>Vault destination</h1>}
              />
            </Routes>
          </TestCryptoProviders>
        </PrivacyProvider>
      </AuthContext.Provider>
    </MemoryRouter>,
  );
}

function responseFor(item = activeItem): RequestImplementation {
  return async (path: string) => {
    if (path === "/v1/vaults/vault-123") {
      return {
        vault,
      };
    }

    return {
      item: encryptedItemResource(item),
    };
  };
}

describe("ItemDetailPage", () => {
  it("shows the parent vault and protects sensitive values", async () => {
    renderItemPage(responseFor());

    expect(
      await screen.findByRole("heading", {
        name: "Test Login",
      }),
    ).toBeInTheDocument();

    expect(
      screen.getByRole("link", {
        name: "Development Vault",
      }),
    ).toBeInTheDocument();

    expect(
      screen.getByRole("link", {
        name: "Back to Vault",
      }),
    ).toBeInTheDocument();

    expect(screen.getByText("••••••••••••")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", {
        name: "Show password",
      }),
    );

    expect(screen.getByText("synthetic-password")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", {
        name: "Copy username",
      }),
    );

    await waitFor(() => {
      expect(writeTextMock).toHaveBeenCalledWith("demo-user");
    });
  });

  it("decrypts an encrypted item detail response", async () => {
    const encryptedItem = encryptedItemResource({
      id: "item-123",
      type: "login",
      payload: {
        title: "Encrypted Test Login",
        username: "encrypted@example.com",
        password: "synthetic-password",
      },
      version: 1,
      createdAt: "2026-07-08T12:00:00Z",
      updatedAt: "2026-07-08T12:00:00Z",
    });

    const requestMock = vi.fn(async (path: string) => {
      if (path === "/v1/vaults/vault-123") {
        return {
          vault,
        };
      }

      return {
        item: encryptedItem,
      };
    });

    renderItemPage(requestMock);

    expect(
      await screen.findByRole("heading", {
        name: "Encrypted Test Login",
      }),
    ).toBeInTheDocument();

    expect(screen.getByText("encrypted@example.com")).toBeInTheDocument();
  });

  it("updates an item through a modal with If-Match", async () => {
    const updatedItem = {
      ...activeItem,
      payload: {
        ...activeItem.payload,
        title: "Updated Login",
      },
      version: 3,
    };

    const requestMock = vi.fn(
      async (path: string, options?: ApiRequestOptions) => {
        if (path === "/v1/vaults/vault-123") {
          return {
            vault,
          };
        }

        if (options?.method === "PUT") {
          return {
            item: encryptedItemResource(updatedItem),
          };
        }

        return {
          item: encryptedItemResource(activeItem),
        };
      },
    );

    renderItemPage(requestMock);

    await screen.findByRole("heading", {
      name: "Test Login",
    });

    fireEvent.click(
      screen.getByRole("button", {
        name: "Edit",
      }),
    );

    const dialog = screen.getByRole("dialog", {
      name: "Edit Item",
    });

    fireEvent.change(within(dialog).getByLabelText("Edit item title"), {
      target: {
        value: "Updated Login",
      },
    });

    fireEvent.click(
      within(dialog).getByRole("button", {
        name: "Save Item",
      }),
    );

    expect(
      await screen.findByRole("heading", {
        name: "Updated Login",
      }),
    ).toBeInTheDocument();

    expect(
      screen.queryByRole("dialog", {
        name: "Edit Item",
      }),
    ).not.toBeInTheDocument();

    const updateCall = requestMock.mock.calls.find(
      ([, options]) => options?.method === "PUT",
    );

    expect(updateCall).toBeDefined();

    expect(updateCall![0]).toBe("/v1/vaults/vault-123/items/item-123");

    expect(updateCall![1]?.json).toEqual({
      type: "login",
      encryptedPayload: {
        version: CRYPTO_ENVELOPE_VERSION,
        algorithm: itemEncryptedPayloadAlgorithm,
        blob: bytesToBase64(validEncryptedBlob()),
      },
    });

    expect("payload" in (updateCall![1]?.json as Record<string, unknown>)).toBe(
      false,
    );

    expect(new Headers(updateCall![1]?.headers).get("If-Match")).toBe('"2"');
  });

  it("requires confirmation before recoverable deletion", async () => {
    const requestMock = vi.fn(
      async (path: string, options?: ApiRequestOptions) => {
        if (path === "/v1/vaults/vault-123") {
          return {
            vault,
          };
        }

        if (options?.method === "DELETE" && !path.endsWith("/permanent")) {
          return {
            item: encryptedItemResource(deletedItem),
          };
        }

        if (path.includes("state=deleted")) {
          return {
            item: encryptedItemResource(deletedItem),
          };
        }

        return {
          item: encryptedItemResource(activeItem),
        };
      },
    );

    renderItemPage(requestMock);

    await screen.findByRole("heading", {
      name: "Test Login",
    });

    fireEvent.click(
      screen.getByRole("button", {
        name: "Delete",
      }),
    );

    const dialog = screen.getByRole("dialog", {
      name: "Delete Item?",
    });

    expect(
      requestMock.mock.calls.some(
        ([, options]) => options?.method === "DELETE",
      ),
    ).toBe(false);

    fireEvent.click(
      within(dialog).getByRole("button", {
        name: "Delete",
      }),
    );

    expect(
      await screen.findByRole("button", {
        name: "Restore",
      }),
    ).toBeInTheDocument();

    const deleteCall = requestMock.mock.calls.find(
      ([path, options]) =>
        path.endsWith("/items/item-123") && options?.method === "DELETE",
    );

    expect(deleteCall).toBeDefined();

    const deleteHeaders = new Headers(deleteCall![1]?.headers);

    expect(deleteHeaders.get("X-VaultForge-Expected-Version")).toBe('"2"');

    expect(deleteHeaders.get("If-Match")).toBeNull();
  });

  it("restores a deleted item", async () => {
    const restoredItem = {
      ...activeItem,
      version: 4,
    };

    let restored = false;

    const requestMock = vi.fn(
      async (path: string, options?: ApiRequestOptions) => {
        if (path === "/v1/vaults/vault-123") {
          return {
            vault,
          };
        }

        if (options?.method === "POST") {
          restored = true;

          return {
            item: encryptedItemResource(restoredItem),
          };
        }

        if (restored && path.includes("state=active")) {
          return {
            item: encryptedItemResource(restoredItem),
          };
        }

        return {
          item: encryptedItemResource(deletedItem),
        };
      },
    );

    renderItemPage(
      requestMock,
      "/vaults/vault-123/items/item-123?state=deleted",
    );

    await screen.findByRole("button", {
      name: "Restore",
    });

    fireEvent.click(
      screen.getByRole("button", {
        name: "Restore",
      }),
    );

    expect(
      await screen.findByRole("button", {
        name: "Edit",
      }),
    ).toBeInTheDocument();

    const restoreCall = requestMock.mock.calls.find(([path]) =>
      path.endsWith("/restore"),
    );

    expect(restoreCall).toBeDefined();

    expect(new Headers(restoreCall![1]?.headers).get("If-Match")).toBe('"3"');
  });

  it("requires confirmation before permanent deletion", async () => {
    const requestMock = vi.fn(
      async (path: string, options?: ApiRequestOptions) => {
        if (path === "/v1/vaults/vault-123") {
          return {
            vault,
          };
        }

        if (path.endsWith("/permanent") && options?.method === "DELETE") {
          return undefined;
        }

        return {
          item: encryptedItemResource(deletedItem),
        };
      },
    );

    renderItemPage(
      requestMock,
      "/vaults/vault-123/items/item-123?state=deleted",
    );

    await screen.findByRole("button", {
      name: "Delete Permanently",
    });

    fireEvent.click(
      screen.getByRole("button", {
        name: "Delete Permanently",
      }),
    );

    const dialog = screen.getByRole("dialog", {
      name: "Permanently Delete Item?",
    });

    expect(
      requestMock.mock.calls.some(
        ([path, options]) =>
          path.endsWith("/permanent") && options?.method === "DELETE",
      ),
    ).toBe(false);

    fireEvent.click(
      within(dialog).getByRole("button", {
        name: "Delete Permanently",
      }),
    );

    expect(
      await screen.findByRole("heading", {
        name: "Vault destination",
      }),
    ).toBeInTheDocument();
  });

  it("does not retry a stale-version edit", async () => {
    const requestMock = vi.fn(
      async (path: string, options?: ApiRequestOptions) => {
        if (path === "/v1/vaults/vault-123") {
          return {
            vault,
          };
        }

        if (options?.method === "PUT") {
          throw new ApiError(
            412,
            "item_version_conflict",
            "The item changed after the supplied version was retrieved.",
            "request-conflict",
          );
        }

        return {
          item: encryptedItemResource(activeItem),
        };
      },
    );

    renderItemPage(requestMock);

    await screen.findByRole("heading", {
      name: "Test Login",
    });

    fireEvent.click(
      screen.getByRole("button", {
        name: "Edit",
      }),
    );

    const dialog = screen.getByRole("dialog", {
      name: "Edit Item",
    });

    fireEvent.change(within(dialog).getByLabelText("Edit item title"), {
      target: {
        value: "Conflicting Update",
      },
    });

    fireEvent.click(
      within(dialog).getByRole("button", {
        name: "Save Item",
      }),
    );

    expect(
      await screen.findByText(
        "The item changed after the supplied version was retrieved.",
      ),
    ).toBeInTheDocument();

    expect(
      screen.getByRole("button", {
        name: "Reload Current Item",
      }),
    ).toBeInTheDocument();

    expect(
      requestMock.mock.calls.filter(([, options]) => options?.method === "PUT"),
    ).toHaveLength(1);
  });

  it("shows an item-not-found state without offering a retry", async () => {
    const requestMock = vi.fn(async (path: string) => {
      if (path === "/v1/vaults/vault-123") {
        return {
          vault,
        };
      }

      throw new ApiError(
        404,
        "item_not_found",
        "The vault item was not found.",
        "request-item-missing",
      );
    });

    renderItemPage(requestMock);

    expect(
      await screen.findByRole("heading", {
        name: "Item Not Found",
      }),
    ).toBeInTheDocument();

    expect(
      screen.getByRole("link", {
        name: "Return to Vault",
      }),
    ).toHaveAttribute("href", "/vaults/vault-123");

    expect(
      screen.queryByRole("button", {
        name: "Try again",
      }),
    ).not.toBeInTheDocument();
  });
});
