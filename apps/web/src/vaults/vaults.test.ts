import { describe, expect, it } from "vitest";

import { ApiError } from "../api/ApiError";
import { parseVaultListResponse, parseVaultResponse } from "./contracts";
import {
  normalizeVaultNameForSubmission,
  validateVaultName,
} from "./validation";

const vaultResource = {
  id: "vault-123",
  name: "Development",
  createdAt: "2026-06-22T12:00:00Z",
  updatedAt: "2026-06-22T12:00:00Z",
};

describe("vault response contracts", () => {
  it("parses a vault response with optional versions", () => {
    const response = parseVaultResponse({
      vault: {
        ...vaultResource,
        cryptoVersion: 1,
        kdfVersion: 2,
      },
    });

    expect(response.vault).toEqual({
      ...vaultResource,
      cryptoVersion: 1,
      kdfVersion: 2,
    });
  });

  it("parses an empty vault list", () => {
    expect(
      parseVaultListResponse({
        vaults: [],
      }),
    ).toEqual({
      vaults: [],
    });
  });

  it("rejects a malformed vault response", () => {
    expect(() =>
      parseVaultResponse({
        vault: {
          name: "Missing ID",
        },
      }),
    ).toThrow(ApiError);
  });

  it("rejects invalid optional version values", () => {
    expect(() =>
      parseVaultResponse({
        vault: {
          ...vaultResource,
          cryptoVersion: "one",
        },
      }),
    ).toThrow(ApiError);
  });
});

describe("vault name validation", () => {
  it("trims and NFC-normalizes a vault name", () => {
    expect(normalizeVaultNameForSubmission("  De\u0301velopment Vault  ")).toBe(
      "Dévelopment Vault",
    );
  });

  it("rejects an empty name", () => {
    expect(validateVaultName("   ")).toBeDefined();
  });

  it("rejects control characters", () => {
    expect(validateVaultName("Development\nVault")).toBeDefined();
  });

  it("rejects names longer than 128 Unicode characters", () => {
    expect(validateVaultName("a".repeat(129))).toBeDefined();
  });

  it("accepts a valid 128-character name", () => {
    expect(validateVaultName("a".repeat(128))).toBeUndefined();
  });
});
