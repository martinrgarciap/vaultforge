import { fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes, useLocation } from "react-router";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "../api/ApiError";
import type { ApiRequestOptions } from "../api/types";
import { AuthContext } from "../auth/AuthContext";
import type { AuthContextValue } from "../auth/types";
import { ItemWorkspace } from "./ItemWorkspace";

const loginItem = {
  id: "login-123",
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

const noteItem = {
  id: "note-123",
  type: "secure_note",
  payload: {
    title: "Synthetic Note",
    note: "Hidden from the table.",
  },
  version: 1,
  createdAt: "2026-06-22T12:00:00Z",
  updatedAt: "2026-06-22T12:45:00Z",
} as const;

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

function LocationResult() {
  const location = useLocation();

  return (
    <p>
      {location.pathname}
      {location.search}
    </p>
  );
}

function renderWorkspace(requestImplementation: RequestImplementation) {
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
    <MemoryRouter initialEntries={["/vaults/vault-123"]}>
      <AuthContext.Provider value={authValue}>
        <Routes>
          <Route
            path="/vaults/:vaultId"
            element={<ItemWorkspace vaultId="vault-123" />}
          />

          <Route
            path="/vaults/:vaultId/items/:itemId"
            element={<LocationResult />}
          />
        </Routes>
      </AuthContext.Provider>
    </MemoryRouter>,
  );
}

describe("ItemWorkspace", () => {
  it("groups nonempty item types and omits empty types", async () => {
    const requestMock = vi.fn(async () => ({
      items: [loginItem, noteItem],
    }));

    renderWorkspace(requestMock);

    expect(
      await screen.findByRole("heading", {
        name: "Logins",
      }),
    ).toBeInTheDocument();

    expect(
      screen.getByRole("heading", {
        name: "Secure Notes",
      }),
    ).toBeInTheDocument();

    expect(
      screen.queryByRole("heading", {
        name: "API Keys",
      }),
    ).not.toBeInTheDocument();

    expect(
      screen.queryByRole("heading", {
        name: "Environment Variables",
      }),
    ).not.toBeInTheDocument();

    expect(
      screen.queryByRole("heading", {
        name: "Database Connections",
      }),
    ).not.toBeInTheDocument();

    const loginRegion = screen.getByRole("region", {
      name: "Logins",
    });

    expect(within(loginRegion).getByText("demo-user")).toBeInTheDocument();

    const noteRegion = screen.getByRole("region", {
      name: "Secure Notes",
    });

    expect(within(noteRegion).getByText("Synthetic Note")).toBeInTheDocument();

    expect(
      within(noteRegion).queryByText("Hidden from the table."),
    ).not.toBeInTheDocument();

    expect(requestMock).toHaveBeenCalledWith(
      "/v1/vaults/vault-123/items?state=active&limit=20",
    );
  });

  it("reveals and copies compact values without opening the row", async () => {
    const requestMock = vi.fn(async () => ({
      items: [loginItem],
    }));

    renderWorkspace(requestMock);

    await screen.findByRole("heading", {
      name: "Logins",
    });

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

    expect(writeTextMock).toHaveBeenCalledWith("demo-user");

    expect(
      screen.queryByText("/vaults/vault-123/items/login-123?state=active"),
    ).not.toBeInTheDocument();
  });

  it("opens an active item from its table row", async () => {
    const requestMock = vi.fn(async () => ({
      items: [loginItem],
    }));

    renderWorkspace(requestMock);

    const row = await screen.findByRole("link", {
      name: "Open Test Login",
    });

    fireEvent.click(row);

    expect(
      await screen.findByText("/vaults/vault-123/items/login-123?state=active"),
    ).toBeInTheDocument();
  });

  it("opens deleted items with deleted state", async () => {
    const deletedItem = {
      ...loginItem,
      version: 3,
      deletedAt: "2026-06-22T13:00:00Z",
    };

    const requestMock = vi.fn(async (path: string) => {
      if (path.includes("state=deleted")) {
        return {
          items: [deletedItem],
        };
      }

      return {
        items: [],
      };
    });

    renderWorkspace(requestMock);

    await screen.findByText("No active items are stored in this vault.");

    fireEvent.click(
      screen.getByRole("button", {
        name: "Deleted Items",
      }),
    );

    const row = await screen.findByRole("link", {
      name: "Open Test Login",
    });

    fireEvent.click(row);

    expect(
      await screen.findByText(
        "/vaults/vault-123/items/login-123?state=deleted",
      ),
    ).toBeInTheDocument();
  });

  it("creates an item through the modal", async () => {
    const createdItem = {
      ...loginItem,
      id: "item-created",
      type: "api_key",
      payload: {
        name: "Created Key",
        service: "Development Service",
        apiKey: "synthetic-value",
      },
      version: 1,
    };

    const requestMock = vi.fn(
      async (_path: string, options?: ApiRequestOptions) => {
        if (options?.method === "POST") {
          return {
            item: createdItem,
          };
        }

        return {
          items: [],
        };
      },
    );

    renderWorkspace(requestMock);

    await screen.findByText("No active items are stored in this vault.");

    fireEvent.click(
      screen.getByRole("button", {
        name: "Create Item",
      }),
    );

    const dialog = screen.getByRole("dialog", {
      name: "Create an item",
    });

    fireEvent.change(within(dialog).getByLabelText("Item type"), {
      target: {
        value: "api_key",
      },
    });

    fireEvent.change(within(dialog).getByLabelText("New item name"), {
      target: {
        value: "Created Key",
      },
    });

    fireEvent.change(within(dialog).getByLabelText("New item service"), {
      target: {
        value: "Development Service",
      },
    });

    fireEvent.change(within(dialog).getByLabelText("New item API key"), {
      target: {
        value: "synthetic-value",
      },
    });

    fireEvent.click(
      within(dialog).getByRole("button", {
        name: "Create item",
      }),
    );

    expect(
      await screen.findByRole("heading", {
        name: "API Keys",
      }),
    ).toBeInTheDocument();

    expect(
      screen.queryByRole("dialog", {
        name: "Create an item",
      }),
    ).not.toBeInTheDocument();

    const createCall = requestMock.mock.calls.find(
      ([, options]) => options?.method === "POST",
    );

    expect(createCall).toBeDefined();

    expect(createCall![0]).toBe("/v1/vaults/vault-123/items");

    expect(createCall![1]?.json).toEqual({
      type: "api_key",
      payload: {
        name: "Created Key",
        service: "Development Service",
        apiKey: "synthetic-value",
      },
    });

    expect(new Headers(createCall![1]?.headers).get("Idempotency-Key")).toMatch(
      /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i,
    );
  });

  it("loads another keyset page", async () => {
    const secondItem = {
      ...loginItem,
      id: "login-456",
      payload: {
        ...loginItem.payload,
        title: "Second Login",
      },
    };

    const requestMock = vi.fn(async (path: string) => {
      if (path.includes("after=cursor-token")) {
        return {
          items: [secondItem],
        };
      }

      return {
        items: [loginItem],
        nextCursor: "cursor-token",
      };
    });

    renderWorkspace(requestMock);

    await screen.findByRole("link", {
      name: "Open Test Login",
    });

    fireEvent.click(
      screen.getByRole("button", {
        name: "Load more",
      }),
    );

    expect(
      await screen.findByRole("link", {
        name: "Open Second Login",
      }),
    ).toBeInTheDocument();
  });

  it("does not show an empty state when initial loading fails", async () => {
    const requestMock = vi
      .fn()
      .mockRejectedValueOnce(
        new ApiError(
          503,
          "item_unavailable",
          "Vault item operations are temporarily unavailable.",
        ),
      )
      .mockResolvedValueOnce({
        items: [],
      });

    renderWorkspace(requestMock);

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Vault item operations are temporarily unavailable.",
    );

    expect(
      screen.queryByText("No active items are stored in this vault."),
    ).not.toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", {
        name: "Try again",
      }),
    );

    expect(
      await screen.findByText("No active items are stored in this vault."),
    ).toBeInTheDocument();

    expect(requestMock).toHaveBeenCalledTimes(2);
  });
});
