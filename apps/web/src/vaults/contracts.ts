import { ApiError } from "../api/ApiError";

export const currentVaultCryptoVersion = 1;
export const currentVaultKdfVersion = 1;

export interface Vault {
  id: string;
  name: string;
  cryptoVersion?: number;
  kdfVersion?: number;
  salt?: string;
  wrappedKey?: string;
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

export interface InitializeVaultCryptoRequest {
  cryptoVersion: typeof currentVaultCryptoVersion;
  kdfVersion: typeof currentVaultKdfVersion;
  salt: string;
  wrappedKey: string;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function requireString(value: Record<string, unknown>, field: string): string {
  const fieldValue = value[field];

  if (typeof fieldValue !== "string" || fieldValue.length === 0) {
    throw ApiError.invalidResponse();
  }

  return fieldValue;
}

function optionalString(
  value: Record<string, unknown>,
  field: string,
): string | undefined {
  const fieldValue = value[field];

  if (fieldValue === undefined) {
    return undefined;
  }

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

function assertConsistentVaultCryptoMetadata(vault: Vault): void {
  const metadata = [
    vault.cryptoVersion,
    vault.kdfVersion,
    vault.salt,
    vault.wrappedKey,
  ];
  const metadataFields = metadata.filter((value) => value !== undefined);

  if (
    metadataFields.length !== 0 &&
    metadataFields.length !== metadata.length
  ) {
    throw ApiError.invalidResponse();
  }
}

function parseVault(value: unknown): Vault {
  if (!isRecord(value)) {
    throw ApiError.invalidResponse();
  }

  const vault: Vault = {
    id: requireString(value, "id"),
    name: requireString(value, "name"),
    cryptoVersion: optionalInt16(value, "cryptoVersion"),
    kdfVersion: optionalInt16(value, "kdfVersion"),
    salt: optionalString(value, "salt"),
    wrappedKey: optionalString(value, "wrappedKey"),
    createdAt: requireString(value, "createdAt"),
    updatedAt: requireString(value, "updatedAt"),
  };

  assertConsistentVaultCryptoMetadata(vault);

  return vault;
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
