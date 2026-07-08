import { describe, expect, it, vi } from "vitest";

import { ApiError } from "../api/ApiError";
import type { CryptoProvider } from "../crypto/CryptoProvider";
import {
  CRYPTO_ENVELOPE_VERSION,
  type ItemCryptoEnvelope,
  type WrappedKeyEnvelope,
} from "../crypto/cryptoTypes";
import { bytesToBase64 } from "../crypto/encoding";
import {
  itemEncryptedPayloadAlgorithm,
  minimumItemEncryptedPayloadBlobBytes,
  type ItemEncryptedPayloadWire,
} from "./encryptedPayload";
import {
  decodeItemPlaintext,
  decryptItemPayload,
  encodeItemPlaintext,
  encryptItemWriteRequest,
} from "./itemEncryption";

function validEncryptedBlob(): Uint8Array {
  return Uint8Array.from(
    { length: minimumItemEncryptedPayloadBlobBytes + 4 },
    (_, index) => index + 1,
  );
}

function createTestItemEnvelope(
  blob = validEncryptedBlob(),
): ItemCryptoEnvelope {
  return {
    version: CRYPTO_ENVELOPE_VERSION,
    algorithm: itemEncryptedPayloadAlgorithm,
    blob,
  };
}

function createTestWrappedKeyEnvelope(): WrappedKeyEnvelope {
  return {
    version: CRYPTO_ENVELOPE_VERSION,
    algorithm: itemEncryptedPayloadAlgorithm,
    wrappedKey: validEncryptedBlob(),
  };
}

function createTestCryptoProvider(
  overrides: Partial<CryptoProvider> = {},
): CryptoProvider {
  return {
    initialize: vi.fn(async () => undefined),
    generateVaultKey: vi.fn(async () => new Uint8Array(32)),
    deriveKey: vi.fn(async () => new Uint8Array(32)),
    encryptItem: vi.fn(
      async (): Promise<ItemCryptoEnvelope> => createTestItemEnvelope(),
    ),
    decryptItem: vi.fn(async () => encodeItemPlaintext({ title: "Decrypted" })),
    wrapKey: vi.fn(
      async (): Promise<WrappedKeyEnvelope> => createTestWrappedKeyEnvelope(),
    ),
    unwrapKey: vi.fn(async () => new Uint8Array(32)),
    ...overrides,
  };
}

describe("item encryption helpers", () => {
  it("encodes and decodes item plaintext JSON objects", () => {
    const payload = {
      title: "Synthetic login",
      username: "demo@example.com",
      password: "synthetic-password",
    };

    const plaintext = encodeItemPlaintext(payload);
    const decoded = decodeItemPlaintext(plaintext);

    expect(decoded).toEqual(payload);
  });

  it("builds encrypted item write requests without plaintext payload fields", async () => {
    const provider = createTestCryptoProvider();
    const vaultKey = Uint8Array.from({ length: 32 }, (_, index) => index + 1);
    const payload = {
      title: "Synthetic API key",
      service: "Example Service",
      apiKey: "vf_synthetic_key",
    };

    const request = await encryptItemWriteRequest({
      provider,
      vaultKey,
      type: "api_key",
      payload,
    });

    expect(request).toEqual({
      type: "api_key",
      encryptedPayload: {
        version: CRYPTO_ENVELOPE_VERSION,
        algorithm: itemEncryptedPayloadAlgorithm,
        blob: bytesToBase64(validEncryptedBlob()),
      },
    });
    expect("payload" in request).toBe(false);
    expect(provider.encryptItem).toHaveBeenCalledTimes(1);

    const [, plaintext] = vi.mocked(provider.encryptItem).mock.calls[0];

    expect(decodeItemPlaintext(plaintext)).toEqual(payload);
  });

  it("passes an independent vault key copy to the crypto provider", async () => {
    const provider = createTestCryptoProvider({
      encryptItem: vi.fn(async (key): Promise<ItemCryptoEnvelope> => {
        key[0] = 255;
        return createTestItemEnvelope();
      }),
    });
    const vaultKey = Uint8Array.from({ length: 32 }, (_, index) => index + 1);

    await encryptItemWriteRequest({
      provider,
      vaultKey,
      type: "secure_note",
      payload: {
        note: "Synthetic note",
      },
    });

    expect(vaultKey[0]).toBe(1);
  });

  it("decrypts encrypted payload envelopes into item payload objects", async () => {
    const provider = createTestCryptoProvider();
    const encryptedPayload: ItemEncryptedPayloadWire = {
      version: CRYPTO_ENVELOPE_VERSION,
      algorithm: itemEncryptedPayloadAlgorithm,
      blob: bytesToBase64(validEncryptedBlob()),
    };

    const payload = await decryptItemPayload({
      provider,
      vaultKey: new Uint8Array(32),
      encryptedPayload,
    });

    expect(payload).toEqual({
      title: "Decrypted",
    });
    expect(provider.decryptItem).toHaveBeenCalledTimes(1);
  });

  it("rejects decrypted plaintext that is not valid JSON", () => {
    expect(() =>
      decodeItemPlaintext(new TextEncoder().encode("not json")),
    ).toThrow(ApiError);
  });

  it("rejects decrypted plaintext that is not a JSON object", () => {
    expect(() =>
      decodeItemPlaintext(new TextEncoder().encode('["not", "object"]')),
    ).toThrow(ApiError);
  });
});
