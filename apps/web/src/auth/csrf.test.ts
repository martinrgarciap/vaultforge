import { afterEach, describe, expect, it } from "vitest";

import { readCSRFToken } from "./csrf";

const validCSRFToken = "vf_csrf_abcdefghijklmnopqrstuvwxyzABCDEFGH123456789";

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

afterEach(() => {
  clearCookies();
});

describe("readCSRFToken", () => {
  it("reads the exact CSRF cookie", () => {
    document.cookie = `vaultforge_csrf=${validCSRFToken}; Path=/`;

    expect(readCSRFToken()).toBe(validCSRFToken);
  });

  it("ignores similarly named cookies", () => {
    document.cookie = `vaultforge_csrf_backup=${validCSRFToken}; Path=/`;

    expect(readCSRFToken()).toBeNull();
  });

  it("decodes an encoded cookie value", () => {
    document.cookie = `vaultforge_csrf=${encodeURIComponent(validCSRFToken)}; Path=/`;

    expect(readCSRFToken()).toBe(validCSRFToken);
  });

  it("returns null for a malformed token", () => {
    document.cookie = "vaultforge_csrf=invalid; Path=/";

    expect(readCSRFToken()).toBeNull();
  });

  it("returns null when the cookie is missing", () => {
    expect(readCSRFToken()).toBeNull();
  });
});
