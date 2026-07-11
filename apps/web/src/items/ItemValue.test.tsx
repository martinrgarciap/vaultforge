import { act, fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { PrivacyContextValue } from "../privacy/PrivacyContext";
import { PrivacyContext } from "../privacy/PrivacyContext";
import { ItemValue } from "./ItemValue";

function renderItemValue(
  privacyValue: Partial<PrivacyContextValue> = {},
  itemProps: Partial<Parameters<typeof ItemValue>[0]> = {},
) {
  const copyValue = vi.fn(async () => true);
  const value: PrivacyContextValue = {
    resetVersion: 0,
    copyValue,
    ...privacyValue,
  };

  const props = {
    label: "Password",
    value: "synthetic-password",
    sensitive: true,
    copyable: true,
    ...itemProps,
  };

  const result = render(
    <PrivacyContext.Provider value={value}>
      <ItemValue {...props} />
    </PrivacyContext.Provider>,
  );

  return {
    ...result,
    copyValue,
  };
}

describe("ItemValue", () => {
  it("masks sensitive values initially", () => {
    renderItemValue();

    expect(screen.getByText("••••••••••••")).toBeInTheDocument();
    expect(screen.queryByText("synthetic-password")).not.toBeInTheDocument();
  });

  it("reveals sensitive values explicitly", () => {
    renderItemValue();

    fireEvent.click(screen.getByRole("button", { name: "Show password" }));

    expect(screen.getByText("synthetic-password")).toBeInTheDocument();
  });

  it("hides revealed sensitive values explicitly", () => {
    renderItemValue();

    fireEvent.click(screen.getByRole("button", { name: "Show password" }));
    fireEvent.click(screen.getByRole("button", { name: "Hide password" }));

    expect(screen.getByText("••••••••••••")).toBeInTheDocument();
  });

  it("automatically hides a revealed value after 15 seconds", () => {
    vi.useFakeTimers();

    try {
      renderItemValue();

      fireEvent.click(screen.getByRole("button", { name: "Show password" }));

      expect(screen.getByText("synthetic-password")).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "Hide password" }),
      ).toBeInTheDocument();

      act(() => {
        vi.advanceTimersByTime(14_999);
      });

      expect(screen.getByText("synthetic-password")).toBeInTheDocument();

      act(() => {
        vi.advanceTimersByTime(1);
      });

      expect(screen.queryByText("synthetic-password")).not.toBeInTheDocument();
      expect(screen.getByText("••••••••••••")).toBeInTheDocument();
      expect(
        screen.getByRole("button", { name: "Show password" }),
      ).toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("remasks revealed values after a privacy reset", () => {
    const { rerender, copyValue } = renderItemValue();

    fireEvent.click(screen.getByRole("button", { name: "Show password" }));
    expect(screen.getByText("synthetic-password")).toBeInTheDocument();

    rerender(
      <PrivacyContext.Provider
        value={{
          resetVersion: 1,
          copyValue,
        }}
      >
        <ItemValue
          label="Password"
          value="synthetic-password"
          sensitive
          copyable
        />
      </PrivacyContext.Provider>,
    );

    expect(screen.getByText("••••••••••••")).toBeInTheDocument();
  });

  it("uses the managed privacy copy function", () => {
    const { copyValue } = renderItemValue();

    fireEvent.click(screen.getByRole("button", { name: "Copy password" }));

    expect(copyValue).toHaveBeenCalledWith({
      label: "Password",
      value: "synthetic-password",
    });
  });

  it("omits copy and reveal controls for an empty value", () => {
    renderItemValue({}, { value: "" });

    expect(screen.getByText("Not provided")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Show password" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Copy password" }),
    ).not.toBeInTheDocument();
  });

  it("temporarily shows a check icon after copying succeeds", async () => {
    vi.useFakeTimers();

    try {
      renderItemValue();

      expect(screen.getByTitle("Copy Password")).toBeInTheDocument();

      await act(async () => {
        fireEvent.click(screen.getByRole("button", { name: "Copy password" }));
        await Promise.resolve();
      });

      expect(screen.getByTitle("Password copied")).toBeInTheDocument();

      act(() => {
        vi.advanceTimersByTime(1200);
      });

      expect(screen.getByTitle("Copy Password")).toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("does not show the check icon when copying fails", async () => {
    const copyValue = vi.fn(async () => false);

    renderItemValue({
      copyValue,
    });

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Copy password" }));
      await Promise.resolve();
    });

    expect(copyValue).toHaveBeenCalled();
    expect(screen.getByTitle("Copy Password")).toBeInTheDocument();
    expect(screen.queryByTitle("Password copied")).not.toBeInTheDocument();
  });

  it("shows an open-website button for a valid website value", () => {
    renderItemValue(
      {},
      {
        label: "Website",
        value: "https://example.test/path",
        sensitive: false,
        copyable: false,
        link: true,
      },
    );

    const anchor = screen.getByRole("link", {
      name: "Open website",
    });

    expect(anchor).toHaveAttribute("href", "https://example.test/path");
    expect(anchor).toHaveAttribute("target", "_blank");
    expect(anchor).toHaveAttribute("rel", "noopener noreferrer");
    expect(screen.getByText("https://example.test/path")).toBeInTheDocument();
  });

  it("does not link an unsafe or non-URL value", () => {
    renderItemValue(
      {},
      {
        label: "Website",
        value: "javascript:alert(1)",
        sensitive: false,
        copyable: false,
        link: true,
      },
    );

    expect(screen.getByText("javascript:alert(1)")).toBeInTheDocument();
    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });

  it("never links a masked sensitive value", () => {
    renderItemValue(
      {},
      {
        label: "Website",
        value: "https://example.test",
        sensitive: true,
        copyable: false,
        link: true,
      },
    );

    expect(screen.queryByRole("link")).not.toBeInTheDocument();
  });
});
