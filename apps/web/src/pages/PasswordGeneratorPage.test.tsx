import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { PasswordGeneratorPage } from "./PasswordGeneratorPage";

function jsonResponse(value: unknown, status = 200): Response {
  return new Response(JSON.stringify(value), {
    status,
    headers: {
      "Content-Type": "application/json",
    },
  });
}

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

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe("PasswordGeneratorPage", () => {
  it("generates a password through the public password endpoint", async () => {
    const fetchMock = vi.fn<typeof fetch>().mockResolvedValue(
      jsonResponse({
        password: "Generated-Demo-Password-123!",
        entropyBits: 142.75,
      }),
    );

    vi.stubGlobal("fetch", fetchMock);

    render(<PasswordGeneratorPage />);

    fireEvent.change(screen.getByLabelText("Length"), {
      target: {
        value: "32",
      },
    });

    fireEvent.change(screen.getByLabelText("Excluded characters"), {
      target: {
        value: "O0l1",
      },
    });

    fireEvent.click(
      screen.getByRole("button", {
        name: "Generate password",
      }),
    );

    expect(
      await screen.findByText("Generated-Demo-Password-123!"),
    ).toBeInTheDocument();

    expect(
      screen.getByText("Estimated entropy: 142.8 bits"),
    ).toBeInTheDocument();

    expect(fetchMock).toHaveBeenCalledTimes(1);

    const [path, request] = fetchMock.mock.calls[0];

    expect(path).toBe("/v1/passwords/generate");
    expect(request?.method).toBe("POST");
    expect(request?.credentials).toBe("include");
    expect(request?.body).toBe(
      '{"length":32,"includeUppercase":true,"includeLowercase":true,"includeDigits":true,"includeSymbols":true,"excludeChars":"O0l1"}',
    );
  });

  it("copies the generated password", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValue(
        jsonResponse({
          password: "Generated-Demo-Password-123!",
          entropyBits: 120,
        }),
      ),
    );

    render(<PasswordGeneratorPage />);

    fireEvent.click(
      screen.getByRole("button", {
        name: "Generate password",
      }),
    );

    await screen.findByText("Generated-Demo-Password-123!");

    fireEvent.click(
      screen.getByRole("button", {
        name: "Copy",
      }),
    );

    expect(writeTextMock).toHaveBeenCalledWith("Generated-Demo-Password-123!");

    expect(
      await screen.findByText("Copied generated password."),
    ).toBeInTheDocument();
  });

  it("requires at least one character set", () => {
    render(<PasswordGeneratorPage />);

    fireEvent.click(screen.getByLabelText("Uppercase letters"));
    fireEvent.click(screen.getByLabelText("Lowercase letters"));
    fireEvent.click(screen.getByLabelText("Digits"));
    fireEvent.click(screen.getByLabelText("Symbols"));

    expect(
      screen.getByText("Select at least one character set."),
    ).toBeInTheDocument();

    expect(
      screen.getByRole("button", {
        name: "Generate password",
      }),
    ).toBeDisabled();
  });

  it("displays API errors safely", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn<typeof fetch>().mockResolvedValue(
        jsonResponse(
          {
            error: {
              code: "password_tools_unavailable",
              message: "Password tools are temporarily unavailable.",
              request_id: "request-password-123",
            },
          },
          503,
        ),
      ),
    );

    render(<PasswordGeneratorPage />);

    fireEvent.click(
      screen.getByRole("button", {
        name: "Generate password",
      }),
    );

    const alert = await screen.findByRole("alert");

    expect(alert).toHaveTextContent(
      "Password tools are temporarily unavailable.",
    );
    expect(alert).toHaveTextContent("Request ID: request-password-123");
  });
});
