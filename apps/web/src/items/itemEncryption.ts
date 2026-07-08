import { ApiError } from "../api/ApiError";
import type { CryptoProvider } from "../crypto/CryptoProvider";
import type {
  DecryptedItemPayload,
  EncryptedItemWriteRequest,
  ItemType,
} from "./contracts";
import type { ItemEncryptedPayloadWire } from "./encryptedPayload";
import {
  itemCryptoEnvelopeFromWire,
  itemCryptoEnvelopeToWire,
} from "./encryptedPayload";

const textEncoder = new TextEncoder();
const textDecoder = new TextDecoder("utf-8", { fatal: true });

export interface EncryptItemWriteRequestInput {
  provider: CryptoProvider;
  vaultKey: Uint8Array;
  type: ItemType;
  payload: DecryptedItemPayload;
}

export interface DecryptItemPayloadInput {
  provider: CryptoProvider;
  vaultKey: Uint8Array;
  encryptedPayload: ItemEncryptedPayloadWire;
}

export async function encryptItemWriteRequest({
  provider,
  vaultKey,
  type,
  payload,
}: EncryptItemWriteRequestInput): Promise<EncryptedItemWriteRequest> {
  const plaintext = encodeItemPlaintext(payload);
  const envelope = await provider.encryptItem(
    cloneVaultKey(vaultKey),
    plaintext,
  );

  return {
    type,
    encryptedPayload: itemCryptoEnvelopeToWire(envelope),
  };
}

export async function decryptItemPayload({
  provider,
  vaultKey,
  encryptedPayload,
}: DecryptItemPayloadInput): Promise<DecryptedItemPayload> {
  const envelope = itemCryptoEnvelopeFromWire(encryptedPayload);
  const plaintext = await provider.decryptItem(
    cloneVaultKey(vaultKey),
    envelope,
  );

  return decodeItemPlaintext(plaintext);
}

export function encodeItemPlaintext(payload: DecryptedItemPayload): Uint8Array {
  if (!isRecord(payload)) {
    throw new TypeError("Item payload must be a JSON object.");
  }

  return textEncoder.encode(JSON.stringify(payload));
}

export function decodeItemPlaintext(
  plaintext: Uint8Array,
): DecryptedItemPayload {
  let decoded: string;

  try {
    decoded = textDecoder.decode(plaintext);
  } catch {
    throw ApiError.invalidResponse();
  }

  let parsed: unknown;

  try {
    parsed = JSON.parse(decoded);
  } catch {
    throw ApiError.invalidResponse();
  }

  if (!isRecord(parsed)) {
    throw ApiError.invalidResponse();
  }

  return parsed;
}

function cloneVaultKey(vaultKey: Uint8Array): Uint8Array {
  return new Uint8Array(vaultKey);
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
