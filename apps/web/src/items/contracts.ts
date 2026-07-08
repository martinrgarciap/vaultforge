import { ApiError } from "../api/ApiError";
import type { ItemEncryptedPayloadWire } from "./encryptedPayload";

export const itemTypes = [
  "login",
  "api_key",
  "environment_variable",
  "database_connection",
  "secure_note",
] as const;

export type ItemType = (typeof itemTypes)[number];
export type ItemState = "active" | "deleted";
export type SyntheticItemPayload = Record<string, unknown>;
export type DecryptedItemPayload = SyntheticItemPayload;

export interface VaultItem {
  id: string;
  type: ItemType;
  payload: SyntheticItemPayload;
  version: number;
  createdAt: string;
  updatedAt: string;
  deletedAt?: string;
}

export interface ItemResponse {
  item: VaultItem;
}

export interface ItemListResponse {
  items: VaultItem[];
  nextCursor?: string;
}

export interface ItemWriteRequest {
  type: ItemType;
  payload: SyntheticItemPayload;
}

export interface EncryptedVaultItem {
  id: string;
  type: ItemType;
  encryptedPayload: ItemEncryptedPayloadWire;
  version: number;
  createdAt: string;
  updatedAt: string;
  deletedAt?: string;
}

export interface EncryptedItemResponse {
  item: EncryptedVaultItem;
}

export interface EncryptedItemListResponse {
  items: EncryptedVaultItem[];
  nextCursor?: string;
}

export interface EncryptedItemWriteRequest {
  type: ItemType;
  encryptedPayload: ItemEncryptedPayloadWire;
}

const supportedItemTypes = new Set<string>(itemTypes);

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

function parseItemType(value: unknown): ItemType {
  if (typeof value !== "string" || !supportedItemTypes.has(value)) {
    throw ApiError.invalidResponse();
  }

  return value as ItemType;
}

function parsePayload(value: unknown): SyntheticItemPayload {
  if (!isRecord(value)) {
    throw ApiError.invalidResponse();
  }

  return value;
}

function parseVersion(value: unknown): number {
  if (typeof value !== "number" || !Number.isInteger(value) || value < 1) {
    throw ApiError.invalidResponse();
  }

  return value;
}

function parseDeletedAt(value: Record<string, unknown>): string | undefined {
  const deletedAt = value.deletedAt;

  if (deletedAt === undefined) {
    return undefined;
  }

  if (typeof deletedAt !== "string" || deletedAt.length === 0) {
    throw ApiError.invalidResponse();
  }

  return deletedAt;
}

function parseVaultItem(value: unknown): VaultItem {
  if (!isRecord(value)) {
    throw ApiError.invalidResponse();
  }

  return {
    id: requireString(value, "id"),
    type: parseItemType(value.type),
    payload: parsePayload(value.payload),
    version: parseVersion(value.version),
    createdAt: requireString(value, "createdAt"),
    updatedAt: requireString(value, "updatedAt"),
    deletedAt: parseDeletedAt(value),
  };
}

export function parseItemResponse(value: unknown): ItemResponse {
  if (!isRecord(value)) {
    throw ApiError.invalidResponse();
  }

  return {
    item: parseVaultItem(value.item),
  };
}

export function parseItemListResponse(value: unknown): ItemListResponse {
  if (!isRecord(value) || !Array.isArray(value.items)) {
    throw ApiError.invalidResponse();
  }

  const nextCursor = value.nextCursor;

  if (
    nextCursor !== undefined &&
    (typeof nextCursor !== "string" || nextCursor.length === 0)
  ) {
    throw ApiError.invalidResponse();
  }

  return {
    items: value.items.map(parseVaultItem),
    nextCursor,
  };
}
