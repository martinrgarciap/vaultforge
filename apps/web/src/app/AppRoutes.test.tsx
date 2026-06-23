import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";

import { AuthContext } from "../auth/AuthContext";
import type { AuthContextValue } from "../auth/types";
import { AppRoutes } from "./AppRoutes";

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

function renderRoute(
  path: string,
  authOverrides: Partial<AuthContextValue> = {},
) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <AuthContext.Provider value={createAuthValue(authOverrides)}>
        <AppRoutes />
      </AuthContext.Provider>
    </MemoryRouter>,
  );
}

describe("AppRoutes", () => {
  it("redirects the root route to login", async () => {
    renderRoute("/");

    expect(
      await screen.findByRole("heading", {
        name: "Sign in",
      }),
    ).toBeInTheDocument();
  });

  it.each([
    ["/register", "Create your account"],
    ["/login", "Sign in"],
    ["/vaults", "Your Vaults"],
    ["/sessions", "Active sessions"],
  ])("renders %s", (path, heading) => {
    renderRoute(path);

    expect(
      screen.getByRole("heading", {
        name: heading,
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

    expect(requestMock).toHaveBeenCalledWith("/v1/vaults/vault-123");
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
          payload: {
            title: "Synthetic Note",
            note: "Synthetic content.",
          },
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

    expect(
      screen.getByRole("link", {
        name: "Development Vault",
      }),
    ).toBeInTheDocument();

    expect(requestMock).toHaveBeenCalledWith("/v1/vaults/vault-123");

    expect(requestMock).toHaveBeenCalledWith(
      "/v1/vaults/vault-123/items/item-123?state=active",
    );
  });

  it("renders the not-found page for an unknown route", () => {
    renderRoute("/unknown");

    expect(
      screen.getByRole("heading", {
        name: "Page not found",
      }),
    ).toBeInTheDocument();
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
        "Use synthetic data only. Browser-side encryption is not implemented.",
      ),
    ).toBeInTheDocument();
  });
});
