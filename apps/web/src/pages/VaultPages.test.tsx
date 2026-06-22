import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { describe, expect, it, vi } from "vitest";

import { ApiError } from "../api/ApiError";
import type { ApiRequestOptions } from "../api/types";
import { AuthContext } from "../auth/AuthContext";
import type { AuthContextValue, AuthStatus } from "../auth/types";
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
          <Route path="/vaults/:vaultId" element={<VaultDetailPage />} />
          <Route path="/login" element={<h1>Login destination</h1>} />
        </Routes>
      </AuthContext.Provider>
    </MemoryRouter>,
  );
}

function renderVaultDetail(requestImplementation: RequestImplementation) {
  render(
    <MemoryRouter initialEntries={["/vaults/vault-123"]}>
      <AuthContext.Provider value={createAuthValue(requestImplementation)}>
        <Routes>
          <Route path="/vaults/:vaultId" element={<VaultDetailPage />} />
          <Route path="/vaults" element={<h1>Vault list destination</h1>} />
        </Routes>
      </AuthContext.Provider>
    </MemoryRouter>,
  );
}

describe("VaultsPage", () => {
  it("loads and displays owned vaults", async () => {
    const requestMock = vi.fn(async () => ({
      vaults: [vault],
    }));

    renderVaultList(requestMock);

    expect(
      await screen.findByRole("link", {
        name: "Development",
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

  it("creates a normalized vault and adds it to the list", async () => {
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

    fireEvent.change(screen.getByLabelText("Vault name"), {
      target: {
        value: "  De\u0301velopment Vault  ",
      },
    });

    fireEvent.click(
      screen.getByRole("button", {
        name: "Create vault",
      }),
    );

    expect(
      await screen.findByRole("link", {
        name: "Dévelopment Vault",
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

    fireEvent.change(screen.getByLabelText("Vault name"), {
      target: {
        value: "   ",
      },
    });

    fireEvent.click(
      screen.getByRole("button", {
        name: "Create vault",
      }),
    );

    expect(
      screen.getByText(
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
    expect(alert).toHaveTextContent("Request ID: request-123");
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
  it("loads and renames a vault", async () => {
    const requestMock = vi.fn(
      async (_path: string, options?: ApiRequestOptions) => {
        if (options?.method === "PATCH") {
          return {
            vault: {
              ...vault,
              name: "Production",
              updatedAt: "2026-06-22T13:00:00Z",
            },
          };
        }

        return {
          vault,
        };
      },
    );

    renderVaultDetail(requestMock);

    const nameInput = await screen.findByDisplayValue("Development");

    fireEvent.change(nameInput, {
      target: {
        value: "  Production  ",
      },
    });

    fireEvent.click(
      screen.getByRole("button", {
        name: "Save name",
      }),
    );

    await waitFor(() => {
      expect(nameInput).toHaveValue("Production");
    });

    expect(
      screen.getByRole("heading", {
        name: "Production",
      }),
    ).toBeInTheDocument();

    expect(requestMock).toHaveBeenCalledWith("/v1/vaults/vault-123", {
      method: "PATCH",
      json: {
        name: "Production",
      },
    });
  });

  it("requires confirmation before deleting a vault", async () => {
    const requestMock = vi.fn(
      async (_path: string, options?: ApiRequestOptions) => {
        if (options?.method === "DELETE") {
          return undefined;
        }

        return {
          vault,
        };
      },
    );

    renderVaultDetail(requestMock);

    await screen.findByDisplayValue("Development");

    fireEvent.click(
      screen.getByRole("button", {
        name: "Delete vault",
      }),
    );

    expect(
      screen.getByRole("group", {
        name: "Delete vault confirmation",
      }),
    ).toBeInTheDocument();

    expect(requestMock).toHaveBeenCalledTimes(1);

    fireEvent.click(
      screen.getByRole("button", {
        name: "Delete permanently",
      }),
    );

    expect(
      await screen.findByRole("heading", {
        name: "Vault list destination",
      }),
    ).toBeInTheDocument();

    expect(requestMock).toHaveBeenCalledWith("/v1/vaults/vault-123", {
      method: "DELETE",
    });
  });
});
