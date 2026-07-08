import { ApiError } from "../api/ApiError";
import type { CryptoProvider } from "../crypto/CryptoProvider";
import type {
  DecryptedItemPayload,
  ItemListResponse,
  ItemResponse,
  ItemType,
  VaultItem,
} from "./contracts";
import { itemTypes } from "./contracts";
import type { ItemEncryptedPayloadWire } from "./encryptedPayload";
import { parseItemEncryptedPayloadWire } from "./encryptedPayload";
import { decryptItemPayload } from "./itemEncryption";

export interface DecryptItemApiResponseInput {
  provider: CryptoProvider;
  vaultKey: Uint8Array;
  value: unknown;
}

export interface DecryptItemApiListResponseInput {
  provider: CryptoProvider;
  vaultKey: Uint8Array;
  value: unknown;
}

const supportedItemTypes = new Set<string>(itemTypes);

export async function decryptItemApiResponse({
  provider,
  vaultKey,
  value,
}: DecryptItemApiResponseInput): Promise<ItemResponse> {
  if (!isRecord(value)) {
    throw ApiError.invalidResponse();
  }

  return {
    item: await decryptItemApiResource({
      provider,
      vaultKey,
      value: value.item,
    }),
  };
}

export async function decryptItemApiListResponse({
  provider,
  vaultKey,
  value,
}: DecryptItemApiListResponseInput): Promise<ItemListResponse> {
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

  const items = await Promise.all(
    value.items.map((item) =>
      decryptItemApiResource({
        provider,
        vaultKey,
        value: item,
      }),
    ),
  );

  return {
    items,
    nextCursor,
  };
}

async function decryptItemApiResource({
  provider,
  vaultKey,
  value,
}: {
  provider: CryptoProvider;
  vaultKey: Uint8Array;
  value: unknown;
}): Promise<VaultItem> {
  if (!isRecord(value)) {
    throw ApiError.invalidResponse();
  }

  const payload = await parseOrDecryptPayload({
    provider,
    vaultKey,
    value,
  });

  return {
    id: requireString(value, "id"),
    type: parseItemType(value.type),
    payload,
    version: parseVersion(value.version),
    createdAt: requireString(value, "createdAt"),
    updatedAt: requireString(value, "updatedAt"),
    deletedAt: parseDeletedAt(value),
  };
}

async function parseOrDecryptPayload({
  provider,
  vaultKey,
  value,
}: {
  provider: CryptoProvider;
  vaultKey: Uint8Array;
  value: Record<string, unknown>;
}): Promise<DecryptedItemPayload> {
  const hasPlaintextPayload = Object.prototype.hasOwnProperty.call(
    value,
    "payload",
  );
  const hasEncryptedPayload = Object.prototype.hasOwnProperty.call(
    value,
    "encryptedPayload",
  );

  if (hasPlaintextPayload === hasEncryptedPayload) {
    throw ApiError.invalidResponse();
  }

  if (hasPlaintextPayload) {
    return parsePlaintextPayload(value.payload);
  }

  return decryptItemPayload({
    provider,
    vaultKey,
    encryptedPayload: parseEncryptedPayload(value.encryptedPayload),
  });
}

function parsePlaintextPayload(value: unknown): DecryptedItemPayload {
  if (!isRecord(value)) {
    throw ApiError.invalidResponse();
  }

  return value;
}

function parseEncryptedPayload(value: unknown): ItemEncryptedPayloadWire {
  return parseItemEncryptedPayloadWire(value);
}

function parseItemType(value: unknown): ItemType {
  if (typeof value !== "string" || !supportedItemTypes.has(value)) {
    throw ApiError.invalidResponse();
  }

  return value as ItemType;
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

function requireString(value: Record<string, unknown>, field: string): string {
  const fieldValue = value[field];

  if (typeof fieldValue !== "string" || fieldValue.length === 0) {
    throw ApiError.invalidResponse();
  }

  return fieldValue;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
