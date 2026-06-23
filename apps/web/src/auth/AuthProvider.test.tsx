import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { Account } from "../api/types";
import { AuthProvider } from "./AuthProvider";
import { useAuth } from "./useAuth";

const csrfToken = "vf_csrf_abcdefghijklmnopqrstuvwxyzABCDEFGH123456789";

const account: Account = {
  id: "user-123",
  email: "developer@example.com",
  status: "active",
  createdAt: "2026-06-22T12:00:00Z",
  updatedAt: "2026-06-22T12:00:00Z",
};

function jsonResponse(value: unknown, status = 200): Response {
  return new Response(JSON.stringify(value), {
    status,
    headers: {
      "Content-Type": "application/json",
    },
  });
}

function loginResponse(accessToken = "login-access-token") {
  return {
    user: account,
    tokenType: "Bearer",
    accessToken,
    accessTokenExpiresAt: "2026-06-22T12:10:00Z",
    refreshTokenExpiresAt: "2026-07-22T12:00:00Z",
  };
}

function refreshResponse(accessToken = "refreshed-access-token") {
  return {
    tokenType: "Bearer",
    accessToken,
    accessTokenExpiresAt: "2026-06-22T12:10:00Z",
    refreshTokenExpiresAt: "2026-07-22T12:00:00Z",
  };
}

function unauthorizedResponse(): Response {
  return jsonResponse(
    {
      error: {
        code: "unauthorized",
        message: "A valid access token is required.",
      },
    },
    401,
  );
}

function clearCookies(): void {
  for (const cookie of document.cookie.split(";")) {
    const separatorIndex = cookie.indexOf("=");
    const name =
      separatorIndex >= 0
        ? cookie.slice(0, separatorIndex).trim()
        : cookie.trim();

    if (name) {
      document.cookie = `${name}=; Max-Age=0; Path=/`;
    }
  }
}

function TestConsumer() {
  const auth = useAuth();
  const [result, setResult] = useState("none");

  const login = () => {
    void auth
      .login({
        email: "developer@example.com",
        password: "correct horse battery staple",
      })
      .catch(() => undefined);
  };

  const requestVaults = () => {
    void auth
      .request<{ value: string }>("/v1/vaults")
      .then((response) => {
        setResult(response.value);
      })
      .catch((error: unknown) => {
        if (
          typeof error === "object" &&
          error !== null &&
          "code" in error &&
          typeof error.code === "string"
        ) {
          setResult(error.code);
          return;
        }

        setResult("error");
      });
  };

  const requestTwice = () => {
    void Promise.all([
      auth.request<{ value: string }>("/v1/first"),
      auth.request<{ value: string }>("/v1/second"),
    ])
      .then(([first, second]) => {
        setResult(`${first.value},${second.value}`);
      })
      .catch(() => {
        setResult("error");
      });
  };

  const logout = () => {
    void auth.logout().catch(() => undefined);
  };

  const clearLocalAuthentication = () => {
    void auth
      .logout({
        revokeServerSession: false,
      })
      .catch(() => undefined);
  };

  return (
    <>
      <div data-testid="status">{auth.status}</div>
      <div data-testid="account">{auth.account?.email ?? "none"}</div>
      <div data-testid="result">{result}</div>

      <button type="button" onClick={login}>
        Login
      </button>

      <button type="button" onClick={requestVaults}>
        Request vaults
      </button>

      <button type="button" onClick={requestTwice}>
        Request twice
      </button>

      <button type="button" onClick={logout}>
        Logout
      </button>

      <button type="button" onClick={clearLocalAuthentication}>
        Clear local authentication
      </button>
    </>
  );
}

function renderProvider() {
  render(
    <AuthProvider>
      <TestConsumer />
    </AuthProvider>,
  );
}

afterEach(() => {
  clearCookies();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("AuthProvider", () => {
  it("restores authentication through a bodyless refresh", async () => {
    document.cookie = `vaultforge_csrf=${csrfToken}; Path=/`;

    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValue(jsonResponse(refreshResponse()));

    vi.stubGlobal("fetch", fetchMock);

    renderProvider();

    expect(await screen.findByText("authenticated")).toBeInTheDocument();

    expect(screen.getByTestId("account")).toHaveTextContent("none");

    expect(fetchMock).toHaveBeenCalledTimes(1);

    const [path, request] = fetchMock.mock.calls[0];
    const headers = new Headers(request?.headers);

    expect(path).toBe("/v1/auth/refresh");
    expect(request?.method).toBe("POST");
    expect(request?.credentials).toBe("include");
    expect(request?.body).toBeUndefined();
    expect(headers.get("X-CSRF-Token")).toBe(csrfToken);
    expect(headers.has("Content-Type")).toBe(false);
  });

  it("stores login state in memory", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValue(jsonResponse(loginResponse()));

    vi.stubGlobal("fetch", fetchMock);

    renderProvider();

    expect(screen.getByTestId("status")).toHaveTextContent("unauthenticated");

    fireEvent.click(screen.getByRole("button", { name: "Login" }));

    expect(await screen.findByText("authenticated")).toBeInTheDocument();

    expect(screen.getByTestId("account")).toHaveTextContent(
      "developer@example.com",
    );

    const request = fetchMock.mock.calls[0][1];

    expect(request?.method).toBe("POST");
    expect(request?.body).toBe(
      '{"email":"developer@example.com","password":"correct horse battery staple"}',
    );
  });

  it("refreshes and retries a protected request once", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse(loginResponse()))
      .mockResolvedValueOnce(unauthorizedResponse())
      .mockResolvedValueOnce(jsonResponse(refreshResponse()))
      .mockResolvedValueOnce(
        jsonResponse({
          value: "retried",
        }),
      );

    vi.stubGlobal("fetch", fetchMock);

    renderProvider();

    fireEvent.click(screen.getByRole("button", { name: "Login" }));

    await screen.findByText("authenticated");

    document.cookie = `vaultforge_csrf=${csrfToken}; Path=/`;

    fireEvent.click(
      screen.getByRole("button", {
        name: "Request vaults",
      }),
    );

    expect(await screen.findByText("retried")).toBeInTheDocument();

    expect(fetchMock).toHaveBeenCalledTimes(4);

    const originalHeaders = new Headers(fetchMock.mock.calls[1][1]?.headers);
    const refreshHeaders = new Headers(fetchMock.mock.calls[2][1]?.headers);
    const retryHeaders = new Headers(fetchMock.mock.calls[3][1]?.headers);

    expect(originalHeaders.get("Authorization")).toBe(
      "Bearer login-access-token",
    );
    expect(refreshHeaders.get("X-CSRF-Token")).toBe(csrfToken);
    expect(retryHeaders.get("Authorization")).toBe(
      "Bearer refreshed-access-token",
    );
  });

  it("shares one refresh across simultaneous unauthorized requests", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse(loginResponse()))
      .mockResolvedValueOnce(unauthorizedResponse())
      .mockResolvedValueOnce(unauthorizedResponse())
      .mockResolvedValueOnce(jsonResponse(refreshResponse()))
      .mockResolvedValueOnce(jsonResponse({ value: "first" }))
      .mockResolvedValueOnce(jsonResponse({ value: "second" }));

    vi.stubGlobal("fetch", fetchMock);

    renderProvider();

    fireEvent.click(screen.getByRole("button", { name: "Login" }));

    await screen.findByText("authenticated");

    document.cookie = `vaultforge_csrf=${csrfToken}; Path=/`;

    fireEvent.click(
      screen.getByRole("button", {
        name: "Request twice",
      }),
    );

    expect(await screen.findByText("first,second")).toBeInTheDocument();

    const refreshCalls = fetchMock.mock.calls.filter(
      ([path]) => path === "/v1/auth/refresh",
    );

    expect(refreshCalls).toHaveLength(1);
  });

  it("clears authentication when refresh fails", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse(loginResponse()))
      .mockResolvedValueOnce(unauthorizedResponse())
      .mockResolvedValueOnce(
        jsonResponse(
          {
            error: {
              code: "invalid_refresh_token",
              message: "The refresh token is invalid or expired.",
            },
          },
          401,
        ),
      );

    vi.stubGlobal("fetch", fetchMock);

    renderProvider();

    fireEvent.click(screen.getByRole("button", { name: "Login" }));

    await screen.findByText("authenticated");

    document.cookie = `vaultforge_csrf=${csrfToken}; Path=/`;

    fireEvent.click(
      screen.getByRole("button", {
        name: "Request vaults",
      }),
    );

    expect(await screen.findByText("unauthenticated")).toBeInTheDocument();

    expect(screen.getByTestId("account")).toHaveTextContent("none");
    expect(screen.getByTestId("result")).toHaveTextContent(
      "invalid_refresh_token",
    );
  });

  it("clears local authentication after logout", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse(loginResponse()))
      .mockResolvedValueOnce(
        new Response(null, {
          status: 204,
        }),
      );

    vi.stubGlobal("fetch", fetchMock);

    renderProvider();

    fireEvent.click(screen.getByRole("button", { name: "Login" }));

    await screen.findByText("authenticated");

    fireEvent.click(screen.getByRole("button", { name: "Logout" }));

    await waitFor(() => {
      expect(screen.getByTestId("status")).toHaveTextContent("unauthenticated");
    });

    expect(screen.getByTestId("account")).toHaveTextContent("none");

    const logoutHeaders = new Headers(fetchMock.mock.calls[1][1]?.headers);

    expect(fetchMock.mock.calls[1][0]).toBe("/v1/sessions/current");
    expect(logoutHeaders.get("Authorization")).toBe(
      "Bearer login-access-token",
    );
  });

  it("clears local authentication without another server logout", async () => {
    const fetchMock = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(jsonResponse(loginResponse()));

    vi.stubGlobal("fetch", fetchMock);

    renderProvider();

    fireEvent.click(screen.getByRole("button", { name: "Login" }));

    await screen.findByText("authenticated");

    fireEvent.click(
      screen.getByRole("button", {
        name: "Clear local authentication",
      }),
    );

    await waitFor(() => {
      expect(screen.getByTestId("status")).toHaveTextContent("unauthenticated");
    });

    expect(screen.getByTestId("account")).toHaveTextContent("none");
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });
});
