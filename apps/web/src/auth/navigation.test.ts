import { describe, expect, it } from "vitest";

import { readLoginNavigationState } from "./navigation";

describe("readLoginNavigationState", () => {
  it("reads a safe protected return path", () => {
    expect(
      readLoginNavigationState({
        authenticationRequired: true,
        returnTo: "/vaults/vault-123/items/item-456?state=deleted",
      }),
    ).toEqual({
      registrationComplete: false,
      authenticationRequired: true,
      returnTo: "/vaults/vault-123/items/item-456?state=deleted",
    });
  });

  it("reads successful registration state", () => {
    expect(
      readLoginNavigationState({
        registrationComplete: true,
        email: "developer@example.com",
      }),
    ).toEqual({
      registrationComplete: true,
      email: "developer@example.com",
      authenticationRequired: false,
      returnTo: "/vaults",
    });
  });

  it.each([
    "https://example.com",
    "//example.com",
    "/login",
    "/register",
    123,
    null,
  ])("rejects unsafe return destinations", (returnTo) => {
    expect(
      readLoginNavigationState({
        returnTo,
      }).returnTo,
    ).toBe("/vaults");
  });
});
