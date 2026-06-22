import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import { AuthContext } from "../auth/AuthContext";
import type { AuthContextValue } from "../auth/types";
import { AppRoutes } from "./AppRoutes";

const testAuthValue: AuthContextValue = {
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
};

function renderRoute(path: string) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <AuthContext.Provider value={testAuthValue}>
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
    ["/vaults", "Your vaults"],
    ["/sessions", "Active sessions"],
  ])("renders %s", (path, heading) => {
    renderRoute(path);

    expect(screen.getByRole("heading", { name: heading })).toBeInTheDocument();
  });

  it("renders the selected vault identifier", () => {
    renderRoute("/vaults/vault-123");

    expect(
      screen.getByRole("heading", {
        name: "Vault details",
      }),
    ).toBeInTheDocument();

    expect(screen.getByText("vault-123")).toBeInTheDocument();
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
