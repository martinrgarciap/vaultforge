import { act, fireEvent, render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { describe, expect, it, vi } from "vitest";

import { ApiError } from "../api/ApiError";
import type { Account } from "../api/types";
import { AuthContext } from "../auth/AuthContext";
import type { AuthContextValue } from "../auth/types";
import { LoginPage } from "./LoginPage";
import { RegisterPage } from "./RegisterPage";

const account: Account = {
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

function renderAuthenticationPages(
  authValue: AuthContextValue,
  initialPath: "/login" | "/register",
) {
  render(
    <MemoryRouter initialEntries={[initialPath]}>
      <AuthContext.Provider value={authValue}>
        <Routes>
          <Route path="/login" element={<LoginPage />} />
          <Route path="/register" element={<RegisterPage />} />
          <Route path="/vaults" element={<h1>Vault destination</h1>} />
        </Routes>
      </AuthContext.Provider>
    </MemoryRouter>,
  );
}

function fillLoginForm() {
  fireEvent.change(screen.getByLabelText("Email address"), {
    target: {
      value: "  Developer@Example.COM  ",
    },
  });

  fireEvent.change(screen.getByLabelText("Password"), {
    target: {
      value: "correct horse battery staple",
    },
  });
}

function fillRegistrationForm() {
  fireEvent.change(screen.getByLabelText("Email address"), {
    target: {
      value: "  Developer@Example.COM  ",
    },
  });

  fireEvent.change(screen.getByLabelText("Password"), {
    target: {
      value: "correct horse battery staple",
    },
  });

  fireEvent.change(screen.getByLabelText("Confirm password"), {
    target: {
      value: "correct horse battery staple",
    },
  });
}

describe("LoginPage", () => {
  it("validates required fields before calling login", () => {
    const loginMock = vi.fn(async () => account);

    renderAuthenticationPages(
      createAuthValue({
        login: loginMock,
      }),
      "/login",
    );

    fireEvent.click(
      screen.getByRole("button", {
        name: "Sign in",
      }),
    );

    expect(screen.getByText("Email address is required.")).toBeInTheDocument();
    expect(screen.getByText("Password is required.")).toBeInTheDocument();
    expect(loginMock).not.toHaveBeenCalled();
  });

  it("submits normalized email and redirects to vaults", async () => {
    const loginMock = vi.fn(async () => account);

    renderAuthenticationPages(
      createAuthValue({
        login: loginMock,
      }),
      "/login",
    );

    fillLoginForm();

    fireEvent.click(
      screen.getByRole("button", {
        name: "Sign in",
      }),
    );

    expect(
      await screen.findByRole("heading", {
        name: "Vault destination",
      }),
    ).toBeInTheDocument();

    expect(loginMock).toHaveBeenCalledWith({
      email: "developer@example.com",
      password: "correct horse battery staple",
    });
  });

  it("displays a safe API error and request ID", async () => {
    const loginMock = vi.fn(async () => {
      throw new ApiError(
        401,
        "invalid_credentials",
        "The email or password is incorrect.",
        "request-123",
      );
    });

    renderAuthenticationPages(
      createAuthValue({
        login: loginMock,
      }),
      "/login",
    );

    fillLoginForm();

    fireEvent.click(
      screen.getByRole("button", {
        name: "Sign in",
      }),
    );

    const alert = await screen.findByRole("alert");

    expect(alert).toHaveTextContent("The email or password is incorrect.");
    expect(alert).toHaveTextContent("Request ID: request-123");
  });

  it("prevents duplicate submissions", async () => {
    let resolveLogin: ((resolvedAccount: Account) => void) | undefined;

    const loginMock = vi.fn(
      () =>
        new Promise<Account>((resolve) => {
          resolveLogin = resolve;
        }),
    );

    renderAuthenticationPages(
      createAuthValue({
        login: loginMock,
      }),
      "/login",
    );

    fillLoginForm();

    const submitButton = screen.getByRole("button", {
      name: "Sign in",
    });

    fireEvent.click(submitButton);
    fireEvent.click(submitButton);

    expect(loginMock).toHaveBeenCalledTimes(1);
    expect(
      screen.getByRole("button", {
        name: "Signing in...",
      }),
    ).toBeDisabled();

    const finishLogin = resolveLogin;

    if (!finishLogin) {
      throw new Error("Login promise was not created.");
    }

    await act(async () => {
      finishLogin(account);
    });

    expect(
      await screen.findByRole("heading", {
        name: "Vault destination",
      }),
    ).toBeInTheDocument();
  });
});

describe("RegisterPage", () => {
  it("validates email, password policy, and confirmation", () => {
    const registerMock = vi.fn(async () => account);

    renderAuthenticationPages(
      createAuthValue({
        register: registerMock,
      }),
      "/register",
    );

    fireEvent.change(screen.getByLabelText("Email address"), {
      target: {
        value: "invalid-email",
      },
    });

    fireEvent.change(screen.getByLabelText("Password"), {
      target: {
        value: "too short",
      },
    });

    fireEvent.change(screen.getByLabelText("Confirm password"), {
      target: {
        value: "different password",
      },
    });

    fireEvent.click(
      screen.getByRole("button", {
        name: "Create account",
      }),
    );

    expect(
      screen.getByText("Enter a valid email address."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Password must contain between 15 and 128 characters."),
    ).toBeInTheDocument();
    expect(screen.getByText("Passwords do not match.")).toBeInTheDocument();
    expect(registerMock).not.toHaveBeenCalled();
  });

  it("registers and redirects to login without authenticating", async () => {
    const registerMock = vi.fn(async () => account);
    const loginMock = vi.fn(async () => account);

    renderAuthenticationPages(
      createAuthValue({
        register: registerMock,
        login: loginMock,
      }),
      "/register",
    );

    fillRegistrationForm();

    fireEvent.click(
      screen.getByRole("button", {
        name: "Create account",
      }),
    );

    expect(
      await screen.findByRole("heading", {
        name: "Sign in",
      }),
    ).toBeInTheDocument();

    expect(screen.getByRole("status")).toHaveTextContent(
      "Account created. Sign in to continue.",
    );

    expect(screen.getByLabelText("Email address")).toHaveValue(
      "developer@example.com",
    );

    expect(registerMock).toHaveBeenCalledWith({
      email: "developer@example.com",
      password: "correct horse battery staple",
    });

    expect(loginMock).not.toHaveBeenCalled();
  });

  it("displays registration API errors safely", async () => {
    const registerMock = vi.fn(async () => {
      throw new ApiError(
        409,
        "email_unavailable",
        "An account cannot be created with that email address.",
        "request-456",
      );
    });

    renderAuthenticationPages(
      createAuthValue({
        register: registerMock,
      }),
      "/register",
    );

    fillRegistrationForm();

    fireEvent.click(
      screen.getByRole("button", {
        name: "Create account",
      }),
    );

    const alert = await screen.findByRole("alert");

    expect(alert).toHaveTextContent(
      "An account cannot be created with that email address.",
    );
    expect(alert).toHaveTextContent("Request ID: request-456");
  });
});
