import { fireEvent, render, screen } from "@testing-library/react";
import { useState } from "react";
import { afterEach, describe, expect, it, vi } from "vitest";

import { Modal } from "./Modal";

afterEach(() => {
  document.body.style.overflow = "";
});

describe("Modal", () => {
  it("focuses the close button and locks body scrolling", () => {
    render(
      <Modal title="Test dialog" onClose={() => undefined}>
        <button type="button">Action</button>
      </Modal>,
    );

    expect(
      screen.getByRole("button", {
        name: "Close Test dialog",
      }),
    ).toHaveFocus();
    expect(document.body.style.overflow).toBe("hidden");
  });

  it("closes when Escape is pressed while not busy", () => {
    const onClose = vi.fn();

    render(
      <Modal title="Test dialog" onClose={onClose}>
        <button type="button">Action</button>
      </Modal>,
    );

    fireEvent.keyDown(document, {
      key: "Escape",
    });

    expect(onClose).toHaveBeenCalledTimes(1);
  });

  it("does not close from Escape or the backdrop while busy", () => {
    const onClose = vi.fn();

    render(
      <Modal title="Test dialog" onClose={onClose} isBusy>
        <button type="button" disabled>
          Working
        </button>
      </Modal>,
    );

    fireEvent.keyDown(document, {
      key: "Escape",
    });

    const dialog = screen.getByRole("dialog");
    const backdrop = dialog.parentElement;

    expect(backdrop).not.toBeNull();

    if (backdrop) {
      fireEvent.mouseDown(backdrop);
    }

    expect(onClose).not.toHaveBeenCalled();
  });

  it("wraps forward focus from the last control to the first control", () => {
    render(
      <Modal title="Test dialog" onClose={() => undefined}>
        <button type="button">First action</button>
        <button type="button">Last action</button>
      </Modal>,
    );

    const closeButton = screen.getByRole("button", {
      name: "Close Test dialog",
    });
    const lastButton = screen.getByRole("button", {
      name: "Last action",
    });

    lastButton.focus();

    fireEvent.keyDown(document, {
      key: "Tab",
    });

    expect(closeButton).toHaveFocus();
  });

  it("wraps reverse focus from the first control to the last control", () => {
    render(
      <Modal title="Test dialog" onClose={() => undefined}>
        <button type="button">First action</button>
        <button type="button">Last action</button>
      </Modal>,
    );

    const closeButton = screen.getByRole("button", {
      name: "Close Test dialog",
    });
    const lastButton = screen.getByRole("button", {
      name: "Last action",
    });

    closeButton.focus();

    fireEvent.keyDown(document, {
      key: "Tab",
      shiftKey: true,
    });

    expect(lastButton).toHaveFocus();
  });

  it("restores focus to the element that opened the dialog", () => {
    function ModalHarness() {
      const [isOpen, setIsOpen] = useState(false);

      return (
        <>
          <button
            type="button"
            onClick={() => {
              setIsOpen(true);
            }}
          >
            Open dialog
          </button>

          {isOpen ? (
            <Modal
              title="Test dialog"
              onClose={() => {
                setIsOpen(false);
              }}
            >
              <button type="button">Action</button>
            </Modal>
          ) : null}
        </>
      );
    }

    render(<ModalHarness />);

    const openButton = screen.getByRole("button", {
      name: "Open dialog",
    });

    openButton.focus();
    fireEvent.click(openButton);

    const closeButton = screen.getByRole("button", {
      name: "Close Test dialog",
    });

    expect(closeButton).toHaveFocus();

    fireEvent.click(closeButton);

    expect(openButton).toHaveFocus();
    expect(document.body.style.overflow).toBe("");
  });
});
