import { fireEvent, render, screen, within } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router";
import { describe, expect, it, vi } from "vitest";

import { ApiError } from "../api/ApiError";
import type { ApiRequestOptions } from "../api/types";
import { AuthContext } from "../auth/AuthContext";
import type {
  AuthContextValue,
  AuthStatus,
  LogoutOptions,
} from "../auth/types";
import { SessionsPage } from "./SessionsPage";

const currentSession = {
  id: "session-current",
  userAgent:
    "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 Chrome/148.0.0.0 Safari/537.36",
  createdAt: "2026-06-22T12:00:00Z",
  expiresAt: "2026-07-22T12:00:00Z",
  current: true,
};

const otherSession = {
  id: "session-other",
  userAgent:
    "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:140.0) Gecko/20100101 Firefox/140.0",
  createdAt: "2026-06-21T12:00:00Z",
  expiresAt: "2026-07-21T12:00:00Z",
  current: false,
};

type RequestImplementation = (
  path: string,
  options?: ApiRequestOptions,
) => Promise<unknown>;

function renderSessionsPage(
  requestImplementation: RequestImplementation,
  options: {
    status?: AuthStatus;
    logout?: (options?: LogoutOptions) => Promise<void>;
  } = {},
) {
  const logoutMock = options.logout ?? vi.fn(async () => undefined);

  const authValue: AuthContextValue = {
    status: options.status ?? "authenticated",
    account: null,
    register: async () => {
      throw new Error("Unexpected registration call.");
    },
    login: async () => {
      throw new Error("Unexpected login call.");
    },
    logout: logoutMock,
    request: requestImplementation as AuthContextValue["request"],
  };

  render(
    <MemoryRouter initialEntries={["/sessions"]}>
      <AuthContext.Provider value={authValue}>
        <Routes>
          <Route path="/sessions" element={<SessionsPage />} />
          <Route path="/login" element={<h1>Login destination</h1>} />
        </Routes>
      </AuthContext.Provider>
    </MemoryRouter>,
  );

  return {
    logoutMock,
  };
}

describe("SessionsPage", () => {
  it("lists active sessions and marks the current browser", async () => {
    const requestMock = vi.fn(async () => ({
      sessions: [otherSession, currentSession],
    }));

    renderSessionsPage(requestMock);

    expect(
      await screen.findByRole("heading", {
        name: "Active Sessions",
      }),
    ).toBeInTheDocument();

    expect(screen.getByText("Chrome on macOS")).toBeInTheDocument();
    expect(screen.getByText("Firefox on Windows")).toBeInTheDocument();
    expect(screen.getByText("Current Session")).toBeInTheDocument();

    expect(
      screen.getByRole("button", {
        name: "Log Out This Device",
      }),
    ).toBeInTheDocument();

    expect(
      screen.getByRole("button", {
        name: "Revoke",
      }),
    ).toBeInTheDocument();

    expect(requestMock).toHaveBeenCalledWith("/v1/sessions");
  });

  it("revokes another session after confirmation", async () => {
    const requestMock = vi.fn(
      async (_path: string, options?: ApiRequestOptions) => {
        if (options?.method === "DELETE") {
          return undefined;
        }

        return {
          sessions: [currentSession, otherSession],
        };
      },
    );

    renderSessionsPage(requestMock);

    await screen.findByText("Firefox on Windows");

    fireEvent.click(
      screen.getByRole("button", {
        name: "Revoke",
      }),
    );

    const dialog = screen.getByRole("dialog", {
      name: "Revoke Session?",
    });

    expect(
      requestMock.mock.calls.some(
        ([, options]) => options?.method === "DELETE",
      ),
    ).toBe(false);

    fireEvent.click(
      within(dialog).getByRole("button", {
        name: "Revoke Session",
      }),
    );

    expect(await screen.findByText("Chrome on macOS")).toBeInTheDocument();

    expect(screen.queryByText("Firefox on Windows")).not.toBeInTheDocument();

    expect(requestMock).toHaveBeenCalledWith("/v1/sessions/session-other", {
      method: "DELETE",
    });
  });

  it("logs out the current device after confirmation", async () => {
    const logoutMock = vi.fn(async () => undefined);
    const requestMock = vi.fn(async () => ({
      sessions: [currentSession],
    }));

    renderSessionsPage(requestMock, {
      logout: logoutMock,
    });

    await screen.findByText("Chrome on macOS");

    fireEvent.click(
      screen.getByRole("button", {
        name: "Log Out This Device",
      }),
    );

    const dialog = screen.getByRole("dialog", {
      name: "Log Out This Device?",
    });

    fireEvent.click(
      within(dialog).getByRole("button", {
        name: "Log Out",
      }),
    );

    expect(
      await screen.findByRole("heading", {
        name: "Login destination",
      }),
    ).toBeInTheDocument();

    expect(logoutMock).toHaveBeenCalledWith();
  });

  it("logs out every session without sending a second server logout", async () => {
    const logoutMock = vi.fn(async () => undefined);

    const requestMock = vi.fn(
      async (_path: string, options?: ApiRequestOptions) => {
        if (options?.method === "DELETE") {
          return undefined;
        }

        return {
          sessions: [currentSession, otherSession],
        };
      },
    );

    renderSessionsPage(requestMock, {
      logout: logoutMock,
    });

    await screen.findByText("Chrome on macOS");

    fireEvent.click(
      screen.getByRole("button", {
        name: "Log Out All",
      }),
    );

    const dialog = screen.getByRole("dialog", {
      name: "Log Out All Sessions?",
    });

    fireEvent.click(
      within(dialog).getByRole("button", {
        name: "Log Out All",
      }),
    );

    expect(
      await screen.findByRole("heading", {
        name: "Login destination",
      }),
    ).toBeInTheDocument();

    expect(requestMock).toHaveBeenCalledWith("/v1/sessions", {
      method: "DELETE",
    });

    expect(logoutMock).toHaveBeenCalledWith({
      revokeServerSession: false,
    });
  });

  it("shows safe list errors", async () => {
    const requestMock = vi.fn(async () => {
      throw new ApiError(
        503,
        "authentication_unavailable",
        "Authentication is temporarily unavailable.",
        "request-session-list",
      );
    });

    renderSessionsPage(requestMock);

    const alert = await screen.findByRole("alert");

    expect(alert).toHaveTextContent(
      "Authentication is temporarily unavailable.",
    );
  });

  it("does not request sessions while unauthenticated", () => {
    const requestMock = vi.fn(async () => ({
      sessions: [],
    }));

    renderSessionsPage(requestMock, {
      status: "unauthenticated",
    });

    expect(
      screen.getByText("Sign in to review your active sessions."),
    ).toBeInTheDocument();

    expect(requestMock).not.toHaveBeenCalled();
  });
});
