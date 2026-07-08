import { ApiError } from "../api/ApiError";
import type {
  CryptoEnvelopeVersion,
  ItemCryptoEnvelope,
} from "../crypto/cryptoTypes";
import { CRYPTO_ENVELOPE_VERSION } from "../crypto/cryptoTypes";
import { base64ToBytes, bytesToBase64 } from "../crypto/encoding";

export const itemEncryptedPayloadAlgorithm = "AES-256-GCM";
export const itemEncryptedPayloadNonceBytes = 12;
export const itemEncryptedPayloadTagBytes = 16;
export const minimumItemEncryptedPayloadBlobBytes =
  itemEncryptedPayloadNonceBytes + itemEncryptedPayloadTagBytes;

export interface ItemEncryptedPayloadWire {
  version: CryptoEnvelopeVersion;
  algorithm: typeof itemEncryptedPayloadAlgorithm;
  blob: string;
}

export function itemCryptoEnvelopeToWire(
  envelope: ItemCryptoEnvelope,
): ItemEncryptedPayloadWire {
  assertSupportedEnvelope(envelope);

  return {
    version: CRYPTO_ENVELOPE_VERSION,
    algorithm: itemEncryptedPayloadAlgorithm,
    blob: bytesToBase64(envelope.blob),
  };
}

export function itemCryptoEnvelopeFromWire(value: unknown): ItemCryptoEnvelope {
  const wire = parseItemEncryptedPayloadWire(value);

  return {
    version: CRYPTO_ENVELOPE_VERSION,
    algorithm: itemEncryptedPayloadAlgorithm,
    blob: decodeItemBlob(wire.blob),
  };
}

export function parseItemEncryptedPayloadWire(
  value: unknown,
): ItemEncryptedPayloadWire {
  if (!isRecord(value)) {
    throw ApiError.invalidResponse();
  }

  if (value.version !== CRYPTO_ENVELOPE_VERSION) {
    throw ApiError.invalidResponse();
  }

  if (value.algorithm !== itemEncryptedPayloadAlgorithm) {
    throw ApiError.invalidResponse();
  }

  if (typeof value.blob !== "string" || value.blob.length === 0) {
    throw ApiError.invalidResponse();
  }

  decodeItemBlob(value.blob);

  return {
    version: CRYPTO_ENVELOPE_VERSION,
    algorithm: itemEncryptedPayloadAlgorithm,
    blob: value.blob,
  };
}

function assertSupportedEnvelope(envelope: ItemCryptoEnvelope): void {
  if (
    envelope.version !== CRYPTO_ENVELOPE_VERSION ||
    envelope.algorithm !== itemEncryptedPayloadAlgorithm ||
    !(envelope.blob instanceof Uint8Array) ||
    envelope.blob.length < minimumItemEncryptedPayloadBlobBytes
  ) {
    throw new TypeError("Unsupported encrypted item envelope.");
  }
}

function decodeItemBlob(blob: string): Uint8Array {
  let bytes: Uint8Array;

  try {
    bytes = base64ToBytes(blob, "encrypted payload blob");
  } catch {
    throw ApiError.invalidResponse();
  }

  if (bytes.length < minimumItemEncryptedPayloadBlobBytes) {
    throw ApiError.invalidResponse();
  }

  return bytes;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}
