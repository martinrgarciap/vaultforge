import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { AuthProvider } from "./AuthProvider";
import { useAuth } from "./useAuth";

function jsonResponse(value: unknown): Response {
  return new Response(JSON.stringify(value), {
    status: 201,
    headers: {
      "Content-Type": "application/json",
    },
  });
}

function RegistrationConsumer() {
  const auth = useAuth();
  const [result, setResult] = useState("none");

  const register = () => {
    void auth
      .register({
        email: "developer@example.com",
        password: "correct horse battery staple",
      })
      .then((account) => {
        setResult(account.email);
      });
  };

  return (
    <>
      <div data-testid="status">{auth.status}</div>
      <div data-testid="account">{auth.account?.email ?? "none"}</div>
      <div data-testid="result">{result}</div>

      <button type="button" onClick={register}>
        Register
      </button>
    </>
  );
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("AuthProvider registration", () => {
  it("registers without creating local authenticated state", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      jsonResponse({
        user: {
          id: "user-123",
          email: "developer@example.com",
          status: "active",
          createdAt: "2026-06-22T12:00:00Z",
          updatedAt: "2026-06-22T12:00:00Z",
        },
      }),
    );

    vi.stubGlobal("fetch", fetchMock);

    render(
      <AuthProvider>
        <RegistrationConsumer />
      </AuthProvider>,
    );

    fireEvent.click(
      screen.getByRole("button", {
        name: "Register",
      }),
    );

    expect(
      await screen.findByText("developer@example.com"),
    ).toBeInTheDocument();

    expect(screen.getByTestId("status")).toHaveTextContent("unauthenticated");
    expect(screen.getByTestId("account")).toHaveTextContent("none");

    expect(fetchMock).toHaveBeenCalledTimes(1);

    const [path, request] = fetchMock.mock.calls[0];

    expect(path).toBe("/v1/auth/register");
    expect(request?.method).toBe("POST");
    expect(request?.credentials).toBe("include");
    expect(request?.body).toBe(
      '{"email":"developer@example.com","password":"correct horse battery staple"}',
    );
  });
});
