import { describe, expect, it } from "vitest";

import { ApiError } from "../api/ApiError";
import type { ItemCryptoEnvelope } from "../crypto/cryptoTypes";
import { CRYPTO_ENVELOPE_VERSION } from "../crypto/cryptoTypes";
import { bytesToBase64 } from "../crypto/encoding";
import {
  itemCryptoEnvelopeFromWire,
  itemCryptoEnvelopeToWire,
  itemEncryptedPayloadAlgorithm,
  minimumItemEncryptedPayloadBlobBytes,
  parseItemEncryptedPayloadWire,
} from "./encryptedPayload";

function validBlob(): Uint8Array {
  return Uint8Array.from(
    { length: minimumItemEncryptedPayloadBlobBytes + 4 },
    (_, index) => index % 256,
  );
}

function validEnvelope(): ItemCryptoEnvelope {
  return {
    version: CRYPTO_ENVELOPE_VERSION,
    algorithm: itemEncryptedPayloadAlgorithm,
    blob: validBlob(),
  };
}

describe("encrypted item payload envelopes", () => {
  it("serializes and deserializes a supported crypto envelope", () => {
    const envelope = validEnvelope();

    const wire = itemCryptoEnvelopeToWire(envelope);
    const parsed = itemCryptoEnvelopeFromWire(wire);

    expect(wire).toEqual({
      version: CRYPTO_ENVELOPE_VERSION,
      algorithm: itemEncryptedPayloadAlgorithm,
      blob: expect.any(String),
    });
    expect(parsed.version).toBe(CRYPTO_ENVELOPE_VERSION);
    expect(parsed.algorithm).toBe(itemEncryptedPayloadAlgorithm);
    expect(Array.from(parsed.blob)).toEqual(Array.from(envelope.blob));
  });

  it("parses a valid wire envelope without changing its base64 blob", () => {
    const wire = itemCryptoEnvelopeToWire(validEnvelope());

    expect(parseItemEncryptedPayloadWire(wire)).toEqual(wire);
  });

  it("rejects unsupported envelope versions from API data", () => {
    const wire = itemCryptoEnvelopeToWire(validEnvelope());

    expect(() =>
      itemCryptoEnvelopeFromWire({
        ...wire,
        version: 2,
      }),
    ).toThrow(ApiError);
  });

  it("rejects unsupported algorithms from API data", () => {
    const wire = itemCryptoEnvelopeToWire(validEnvelope());

    expect(() =>
      itemCryptoEnvelopeFromWire({
        ...wire,
        algorithm: "AES-128-GCM",
      }),
    ).toThrow(ApiError);
  });

  it("rejects malformed base64 blobs from API data", () => {
    const wire = itemCryptoEnvelopeToWire(validEnvelope());

    expect(() =>
      itemCryptoEnvelopeFromWire({
        ...wire,
        blob: "not base64",
      }),
    ).toThrow(ApiError);
  });

  it("rejects blobs too short to contain a nonce and authentication tag", () => {
    const shortBlob = bytesToBase64(
      Uint8Array.from({ length: minimumItemEncryptedPayloadBlobBytes - 1 }),
    );
    const wire = itemCryptoEnvelopeToWire(validEnvelope());

    expect(() =>
      itemCryptoEnvelopeFromWire({
        ...wire,
        blob: shortBlob,
      }),
    ).toThrow(ApiError);
  });

  it("rejects unsupported provider envelopes before serialization", () => {
    const envelope = validEnvelope();

    expect(() =>
      itemCryptoEnvelopeToWire({
        ...envelope,
        version: 2 as unknown as 1,
      }),
    ).toThrow(TypeError);
  });
});
