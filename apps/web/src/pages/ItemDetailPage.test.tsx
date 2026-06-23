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
import { PrivacyProvider } from "../privacy/PrivacyProvider";
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
      item,
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
            item: updatedItem,
          };
        }

        return {
          item: activeItem,
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
            item: deletedItem,
          };
        }

        if (path.includes("state=deleted")) {
          return {
            item: deletedItem,
          };
        }

        return {
          item: activeItem,
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

    expect(new Headers(deleteCall![1]?.headers).get("If-Match")).toBe('"2"');
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
            item: restoredItem,
          };
        }

        if (restored && path.includes("state=active")) {
          return {
            item: restoredItem,
          };
        }

        return {
          item: deletedItem,
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
          item: deletedItem,
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
          item: activeItem,
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
