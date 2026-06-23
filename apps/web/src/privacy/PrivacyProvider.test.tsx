import { act, fireEvent, render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { AuthContext } from "../auth/AuthContext";
import type { AuthContextValue, AuthStatus } from "../auth/types";
import { PrivacyProvider } from "./PrivacyProvider";
import { usePrivacy } from "./usePrivacy";

const clipboardClearDelayMs = 50;
const inactivityDelayMs = 100;

function createAuthValue(status: AuthStatus): AuthContextValue {
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
    request: async <T,>() => undefined as T,
  };
}

function PrivacyProbe({ label = "Token", value = "synthetic-token" }) {
  const { copyValue, resetVersion } = usePrivacy();

  return (
    <>
      <p>Reset version: {resetVersion}</p>
      <button
        type="button"
        onClick={() => {
          void copyValue({
            label,
            value,
          });
        }}
      >
        Copy value
      </button>
    </>
  );
}

function renderPrivacy(
  children: ReactNode = <PrivacyProbe />,
  status: AuthStatus = "authenticated",
) {
  return render(
    <AuthContext.Provider value={createAuthValue(status)}>
      <PrivacyProvider
        clipboardClearDelayMs={clipboardClearDelayMs}
        inactivityDelayMs={inactivityDelayMs}
      >
        {children}
      </PrivacyProvider>
    </AuthContext.Provider>,
  );
}

async function flushAsyncWork() {
  await act(async () => {
    await Promise.resolve();
  });
}

function setVisibilityState(value: DocumentVisibilityState) {
  Object.defineProperty(document, "visibilityState", {
    configurable: true,
    value,
  });
}

describe("PrivacyProvider", () => {
  beforeEach(() => {
    vi.useFakeTimers();
    setVisibilityState("visible");
  });

  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("copies and clears matching clipboard text", async () => {
    let clipboardText = "";
    const writeText = vi.fn(async (value: string) => {
      clipboardText = value;
    });
    const readText = vi.fn(async () => clipboardText);

    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        readText,
        writeText,
      },
    });

    renderPrivacy();

    fireEvent.click(screen.getByRole("button", { name: "Copy value" }));
    await flushAsyncWork();

    expect(writeText).toHaveBeenCalledWith("synthetic-token");
    expect(
      screen.getByText(
        "Token copied. VaultForge will attempt to clear it in 30 seconds.",
      ),
    ).toBeInTheDocument();

    await act(async () => {
      vi.advanceTimersByTime(clipboardClearDelayMs);
      await Promise.resolve();
    });

    expect(readText).toHaveBeenCalled();
    expect(writeText).toHaveBeenLastCalledWith("");
    expect(
      screen.getByText("Token was cleared from the clipboard."),
    ).toBeInTheDocument();
  });

  it("shows temporary visible copy confirmation", async () => {
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        readText: vi.fn(async () => "synthetic-token"),
        writeText: vi.fn(async () => undefined),
      },
    });

    renderPrivacy();

    fireEvent.click(screen.getByRole("button", { name: "Copy value" }));
    await flushAsyncWork();

    expect(screen.getByText("Token copied to clipboard.")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(1_999);
    });

    expect(screen.getByText("Token copied to clipboard.")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(1);
    });

    expect(
      screen.queryByText("Token copied to clipboard."),
    ).not.toBeInTheDocument();
  });

  it("does not overwrite changed clipboard text", async () => {
    let clipboardText = "";
    const writeText = vi.fn(async (value: string) => {
      clipboardText = value;
    });

    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        readText: vi.fn(async () => clipboardText),
        writeText,
      },
    });

    renderPrivacy();

    fireEvent.click(screen.getByRole("button", { name: "Copy value" }));
    await flushAsyncWork();

    clipboardText = "newer-external-clipboard-value";

    await act(async () => {
      vi.advanceTimersByTime(clipboardClearDelayMs);
      await Promise.resolve();
    });

    expect(writeText).toHaveBeenCalledTimes(1);
    expect(
      screen.getByText(
        "VaultForge left Token unchanged because the clipboard changed.",
      ),
    ).toBeInTheDocument();
  });

  it("announces when clipboard clearing cannot be scheduled", async () => {
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        writeText: vi.fn(async () => undefined),
      },
    });

    renderPrivacy();

    fireEvent.click(screen.getByRole("button", { name: "Copy value" }));
    await flushAsyncWork();

    expect(
      screen.getByText(
        "Token copied. Automatic clearing is unavailable in this browser.",
      ),
    ).toBeInTheDocument();
  });

  it("announces when clipboard writing is unavailable", async () => {
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {},
    });

    renderPrivacy();

    fireEvent.click(screen.getByRole("button", { name: "Copy value" }));
    await flushAsyncWork();

    expect(
      screen.getByText("Clipboard access is unavailable."),
    ).toBeInTheDocument();
  });

  it("announces initial clipboard write failure", async () => {
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        writeText: vi.fn(async () => {
          throw new Error("write failed");
        }),
      },
    });

    renderPrivacy();

    fireEvent.click(screen.getByRole("button", { name: "Copy value" }));
    await flushAsyncWork();

    expect(
      screen.getByText("The value could not be copied."),
    ).toBeInTheDocument();
  });

  it("announces clipboard read failure during clearing", async () => {
    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        readText: vi.fn(async () => {
          throw new Error("read failed");
        }),
        writeText: vi.fn(async () => undefined),
      },
    });

    renderPrivacy();

    fireEvent.click(screen.getByRole("button", { name: "Copy value" }));
    await flushAsyncWork();

    await act(async () => {
      vi.advanceTimersByTime(clipboardClearDelayMs);
      await Promise.resolve();
    });

    expect(
      screen.getByText(
        "Token was copied, but VaultForge could not clear it automatically.",
      ),
    ).toBeInTheDocument();
  });

  it("announces clipboard clear failure", async () => {
    const writeText = vi
      .fn()
      .mockResolvedValueOnce(undefined)
      .mockRejectedValueOnce(new Error("clear failed"));

    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        readText: vi.fn(async () => "synthetic-token"),
        writeText,
      },
    });

    renderPrivacy();

    fireEvent.click(screen.getByRole("button", { name: "Copy value" }));
    await flushAsyncWork();

    await act(async () => {
      vi.advanceTimersByTime(clipboardClearDelayMs);
      await Promise.resolve();
    });

    expect(
      screen.getByText(
        "Token was copied, but VaultForge could not clear it automatically.",
      ),
    ).toBeInTheDocument();
  });

  it("replaces the pending clear when another value is copied", async () => {
    let clipboardText = "";
    const writeText = vi.fn(async (value: string) => {
      clipboardText = value;
    });

    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        readText: vi.fn(async () => clipboardText),
        writeText,
      },
    });

    const { rerender } = renderPrivacy(
      <PrivacyProbe label="First" value="first" />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Copy value" }));
    await flushAsyncWork();

    rerender(
      <AuthContext.Provider value={createAuthValue("authenticated")}>
        <PrivacyProvider
          clipboardClearDelayMs={clipboardClearDelayMs}
          inactivityDelayMs={inactivityDelayMs}
        >
          <PrivacyProbe label="Second" value="second" />
        </PrivacyProvider>
      </AuthContext.Provider>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Copy value" }));
    await flushAsyncWork();

    await act(async () => {
      vi.advanceTimersByTime(clipboardClearDelayMs);
      await Promise.resolve();
    });

    expect(writeText).toHaveBeenNthCalledWith(1, "first");
    expect(writeText).toHaveBeenNthCalledWith(2, "second");
    expect(writeText).toHaveBeenLastCalledWith("");
    expect(
      screen.getByText("Second was cleared from the clipboard."),
    ).toBeInTheDocument();
  });

  it("cleans up timers and listeners on unmount", async () => {
    const addListener = vi.spyOn(document, "addEventListener");
    const removeListener = vi.spyOn(document, "removeEventListener");
    const writeText = vi.fn(async () => undefined);

    Object.defineProperty(navigator, "clipboard", {
      configurable: true,
      value: {
        readText: vi.fn(async () => "synthetic-token"),
        writeText,
      },
    });

    const { unmount } = renderPrivacy();

    fireEvent.click(screen.getByRole("button", { name: "Copy value" }));
    await flushAsyncWork();

    unmount();

    await act(async () => {
      vi.advanceTimersByTime(clipboardClearDelayMs + inactivityDelayMs);
      await Promise.resolve();
    });

    expect(writeText).toHaveBeenCalledTimes(1);
    expect(addListener).toHaveBeenCalledWith(
      "pointerdown",
      expect.any(Function),
    );
    expect(removeListener).toHaveBeenCalledWith(
      "pointerdown",
      expect.any(Function),
    );
    expect(removeListener).toHaveBeenCalledWith(
      "visibilitychange",
      expect.any(Function),
    );
  });

  it("resets after inactivity", () => {
    renderPrivacy();

    expect(screen.getByText("Reset version: 0")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(inactivityDelayMs);
    });

    expect(screen.getByText("Reset version: 1")).toBeInTheDocument();
    expect(
      screen.getByText(
        "Sensitive values were hidden after inactivity. This does not cryptographically lock the vault.",
      ),
    ).toBeInTheDocument();
  });

  it("restarts inactivity timer after user activity", () => {
    renderPrivacy();

    act(() => {
      vi.advanceTimersByTime(inactivityDelayMs - 1);
    });

    fireEvent.keyDown(document);

    act(() => {
      vi.advanceTimersByTime(1);
    });

    expect(screen.getByText("Reset version: 0")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(inactivityDelayMs);
    });

    expect(screen.getByText("Reset version: 1")).toBeInTheDocument();
  });

  it("resets immediately when the document becomes hidden", () => {
    renderPrivacy();

    setVisibilityState("hidden");
    fireEvent(document, new Event("visibilitychange"));

    expect(screen.getByText("Reset version: 1")).toBeInTheDocument();

    expect(
      screen.getByText(
        "Sensitive values were hidden because the tab was hidden. This does not cryptographically lock the vault.",
      ),
    ).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(inactivityDelayMs);
    });

    expect(screen.getByText("Reset version: 1")).toBeInTheDocument();

    setVisibilityState("visible");
    fireEvent(document, new Event("visibilitychange"));

    act(() => {
      vi.advanceTimersByTime(inactivityDelayMs);
    });

    expect(screen.getByText("Reset version: 2")).toBeInTheDocument();
  });

  it("does not run inactivity timer while unauthenticated", () => {
    renderPrivacy(<PrivacyProbe />, "unauthenticated");

    act(() => {
      vi.advanceTimersByTime(inactivityDelayMs);
    });

    expect(screen.getByText("Reset version: 0")).toBeInTheDocument();
  });
});
