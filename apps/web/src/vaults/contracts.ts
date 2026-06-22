import { ApiError } from "../api/ApiError";

export interface Vault {
  id: string;
  name: string;
  cryptoVersion?: number;
  kdfVersion?: number;
  createdAt: string;
  updatedAt: string;
}

export interface VaultResponse {
  vault: Vault;
}

export interface VaultListResponse {
  vaults: Vault[];
}

export interface CreateVaultRequest {
  name: string;
}

export interface RenameVaultRequest {
  name: string;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function requireString(value: Record<string, unknown>, field: string): string {
  const fieldValue = value[field];

  if (typeof fieldValue !== "string" || fieldValue.length === 0) {
    throw ApiError.invalidResponse();
  }

  return fieldValue;
}

function optionalInt16(
  value: Record<string, unknown>,
  field: string,
): number | undefined {
  const fieldValue = value[field];

  if (fieldValue === undefined) {
    return undefined;
  }

  if (
    typeof fieldValue !== "number" ||
    !Number.isInteger(fieldValue) ||
    fieldValue < -32768 ||
    fieldValue > 32767
  ) {
    throw ApiError.invalidResponse();
  }

  return fieldValue;
}

function parseVault(value: unknown): Vault {
  if (!isRecord(value)) {
    throw ApiError.invalidResponse();
  }

  return {
    id: requireString(value, "id"),
    name: requireString(value, "name"),
    cryptoVersion: optionalInt16(value, "cryptoVersion"),
    kdfVersion: optionalInt16(value, "kdfVersion"),
    createdAt: requireString(value, "createdAt"),
    updatedAt: requireString(value, "updatedAt"),
  };
}

export function parseVaultResponse(value: unknown): VaultResponse {
  if (!isRecord(value)) {
    throw ApiError.invalidResponse();
  }

  return {
    vault: parseVault(value.vault),
  };
}

export function parseVaultListResponse(value: unknown): VaultListResponse {
  if (!isRecord(value) || !Array.isArray(value.vaults)) {
    throw ApiError.invalidResponse();
  }

  return {
    vaults: value.vaults.map(parseVault),
  };
}
