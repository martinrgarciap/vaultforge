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
  decryptItemApiListResponse,
  decryptItemApiResponse,
} from "./itemApiCrypto";
import { encodeItemPlaintext } from "./itemEncryption";

const encryptedBlob = Uint8Array.from(
  { length: minimumItemEncryptedPayloadBlobBytes + 4 },
  (_, index) => index + 1,
);

const encryptedPayload: ItemEncryptedPayloadWire = {
  version: CRYPTO_ENVELOPE_VERSION,
  algorithm: itemEncryptedPayloadAlgorithm,
  blob: bytesToBase64(encryptedBlob),
};

const encryptedItemResource = {
  id: "item-encrypted-123",
  type: "secure_note",
  encryptedPayload,
  version: 1,
  createdAt: "2026-07-08T12:00:00Z",
  updatedAt: "2026-07-08T12:00:00Z",
};

const plaintextItemResource = {
  id: "item-plaintext-123",
  type: "api_key",
  payload: {
    name: "Synthetic API key",
    apiKey: "synthetic-value",
  },
  version: 1,
  createdAt: "2026-07-08T12:00:00Z",
  updatedAt: "2026-07-08T12:00:00Z",
};

function createTestItemEnvelope(): ItemCryptoEnvelope {
  return {
    version: CRYPTO_ENVELOPE_VERSION,
    algorithm: itemEncryptedPayloadAlgorithm,
    blob: encryptedBlob,
  };
}

function createTestWrappedKeyEnvelope(): WrappedKeyEnvelope {
  return {
    version: CRYPTO_ENVELOPE_VERSION,
    algorithm: itemEncryptedPayloadAlgorithm,
    wrappedKey: encryptedBlob,
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
    decryptItem: vi.fn(async () =>
      encodeItemPlaintext({
        title: "Decrypted secure note",
        note: "Synthetic decrypted content.",
      }),
    ),
    wrapKey: vi.fn(
      async (): Promise<WrappedKeyEnvelope> => createTestWrappedKeyEnvelope(),
    ),
    unwrapKey: vi.fn(async () => new Uint8Array(32)),
    ...overrides,
  };
}

describe("encrypted item API response helpers", () => {
  it("decrypts an encrypted item response into a UI VaultItem", async () => {
    const provider = createTestCryptoProvider();

    const response = await decryptItemApiResponse({
      provider,
      vaultKey: new Uint8Array(32),
      value: {
        item: encryptedItemResource,
      },
    });

    expect(response).toEqual({
      item: {
        id: "item-encrypted-123",
        type: "secure_note",
        payload: {
          title: "Decrypted secure note",
          note: "Synthetic decrypted content.",
        },
        version: 1,
        createdAt: "2026-07-08T12:00:00Z",
        updatedAt: "2026-07-08T12:00:00Z",
      },
    });
    expect(provider.decryptItem).toHaveBeenCalledTimes(1);
  });

  it("parses temporary plaintext item responses without decrypting", async () => {
    const provider = createTestCryptoProvider();

    const response = await decryptItemApiResponse({
      provider,
      vaultKey: new Uint8Array(32),
      value: {
        item: plaintextItemResource,
      },
    });

    expect(response).toEqual({
      item: plaintextItemResource,
    });
    expect(provider.decryptItem).not.toHaveBeenCalled();
  });

  it("decrypts encrypted item list responses and preserves cursors", async () => {
    const provider = createTestCryptoProvider();

    const response = await decryptItemApiListResponse({
      provider,
      vaultKey: new Uint8Array(32),
      value: {
        items: [encryptedItemResource],
        nextCursor: "cursor-token",
      },
    });

    expect(response).toEqual({
      items: [
        {
          id: "item-encrypted-123",
          type: "secure_note",
          payload: {
            title: "Decrypted secure note",
            note: "Synthetic decrypted content.",
          },
          version: 1,
          createdAt: "2026-07-08T12:00:00Z",
          updatedAt: "2026-07-08T12:00:00Z",
        },
      ],
      nextCursor: "cursor-token",
    });
  });

  it("rejects item responses with both plaintext and encrypted payloads", async () => {
    const provider = createTestCryptoProvider();

    await expect(
      decryptItemApiResponse({
        provider,
        vaultKey: new Uint8Array(32),
        value: {
          item: {
            ...encryptedItemResource,
            payload: {
              title: "Ambiguous",
            },
          },
        },
      }),
    ).rejects.toThrow(ApiError);
  });

  it("rejects item responses with neither payload shape", async () => {
    const provider = createTestCryptoProvider();

    await expect(
      decryptItemApiResponse({
        provider,
        vaultKey: new Uint8Array(32),
        value: {
          item: {
            id: "item-missing-payload",
            type: "secure_note",
            version: 1,
            createdAt: "2026-07-08T12:00:00Z",
            updatedAt: "2026-07-08T12:00:00Z",
          },
        },
      }),
    ).rejects.toThrow(ApiError);
  });

  it("rejects malformed encrypted payload envelopes", async () => {
    const provider = createTestCryptoProvider();

    await expect(
      decryptItemApiResponse({
        provider,
        vaultKey: new Uint8Array(32),
        value: {
          item: {
            ...encryptedItemResource,
            encryptedPayload: {
              ...encryptedPayload,
              blob: "not base64",
            },
          },
        },
      }),
    ).rejects.toThrow(ApiError);
  });
});
