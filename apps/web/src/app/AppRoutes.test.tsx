import { useCallback, useMemo, useState } from "react";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it, vi } from "vitest";

import { ApiError } from "../api/ApiError";
import { AuthContext } from "../auth/AuthContext";
import type { AuthContextValue, AuthStatus } from "../auth/types";
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
  it("redirects signed-out root visits to login", async () => {
    renderRoute("/");

    expect(
      await screen.findByRole("heading", {
        name: "Sign in",
      }),
    ).toBeInTheDocument();
  });

  it("redirects signed-in root visits to vaults", async () => {
    const requestMock = vi.fn(async () => ({
      vaults: [],
    }));

    renderRoute("/", {
      status: "authenticated",
      request: requestMock as AuthContextValue["request"],
    });

    expect(
      await screen.findByRole("heading", {
        name: "Your Vaults",
      }),
    ).toBeInTheDocument();
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
          <AppRoutes />
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
        "Use synthetic data only. Browser-side encryption is not implemented.",
      ),
    ).toBeInTheDocument();
  });
});
