import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router";
import { describe, expect, it } from "vitest";

import { AppRoutes } from "./AppRoutes";

function renderRoute(path: string) {
  render(
    <MemoryRouter initialEntries={[path]}>
      <AppRoutes />
    </MemoryRouter>,
  );
}

describe("AppRoutes", () => {
  it("redirects the root route to login", async () => {
    renderRoute("/");

    expect(
      await screen.findByRole("heading", { name: "Sign in" }),
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
      screen.getByRole("heading", { name: "Vault details" }),
    ).toBeInTheDocument();

    expect(screen.getByText("vault-123")).toBeInTheDocument();
  });

  it("renders the not-found page for an unknown route", () => {
    renderRoute("/unknown");

    expect(
      screen.getByRole("heading", { name: "Page not found" }),
    ).toBeInTheDocument();
  });

  it("renders the shared application shell", () => {
    renderRoute("/login");

    expect(
      screen.getByRole("navigation", { name: "Primary navigation" }),
    ).toBeInTheDocument();

    expect(
      screen.getByText(
        "Use synthetic data only. Browser-side encryption is not implemented.",
      ),
    ).toBeInTheDocument();
  });
});
