import { describe, expect, it, vi } from "vitest";

import { ApiError } from "../api/ApiError";
import type { ApiRequestOptions } from "../api/types";
import type { CryptoProvider } from "../crypto/CryptoProvider";
import {
  CRYPTO_ENVELOPE_VERSION,
  type ItemCryptoEnvelope,
  type WrappedKeyEnvelope,
} from "../crypto/cryptoTypes";
import { bytesToBase64 } from "../crypto/encoding";
import type { Vault } from "./contracts";
import { currentVaultCryptoVersion, currentVaultKdfVersion } from "./contracts";
import {
  type AuthenticatedRequest,
  isVaultCryptoInitialized,
  requireVaultCryptoMetadata,
  setupVaultCrypto,
  unlockVaultCrypto,
} from "./vaultCrypto";

const baseVault: Vault = {
  id: "vault-123",
  name: "Development",
  createdAt: "2026-06-22T12:00:00Z",
  updatedAt: "2026-06-22T12:00:00Z",
};

const saltBytes = new Uint8Array(32).fill(7);
const wrappedKeyBytes = new Uint8Array(60).fill(9);
const vaultKeyBytes = new Uint8Array(32).fill(5);
const kekBytes = new Uint8Array(32).fill(3);

const initializedVault: Vault = {
  ...baseVault,
  cryptoVersion: currentVaultCryptoVersion,
  kdfVersion: currentVaultKdfVersion,
  salt: bytesToBase64(saltBytes),
  wrappedKey: bytesToBase64(wrappedKeyBytes),
};

interface ProviderCaptures {
  derivePassphrase?: Uint8Array;
  deriveSalt?: Uint8Array;
  wrapVaultKey?: Uint8Array;
  unwrapKek?: Uint8Array;
  unwrapWrappedKey?: Uint8Array;
}

function createProvider(captures: ProviderCaptures = {}): CryptoProvider {
  return {
    initialize: vi.fn(async () => undefined),
    generateVaultKey: vi.fn(async () => new Uint8Array(vaultKeyBytes)),
    deriveKey: vi.fn(async (passphrase, params) => {
      captures.derivePassphrase = new Uint8Array(passphrase);
      captures.deriveSalt = new Uint8Array(params.salt);

      return new Uint8Array(kekBytes);
    }),
    encryptItem: vi.fn(async (): Promise<ItemCryptoEnvelope> => {
      throw new Error("encryptItem is not used by vault crypto tests.");
    }),
    decryptItem: vi.fn(async (): Promise<Uint8Array> => {
      throw new Error("decryptItem is not used by vault crypto tests.");
    }),
    wrapKey: vi.fn(async (_kek, vaultKey): Promise<WrappedKeyEnvelope> => {
      captures.wrapVaultKey = new Uint8Array(vaultKey);

      return {
        version: CRYPTO_ENVELOPE_VERSION,
        algorithm: "AES-256-GCM",
        wrappedKey: new Uint8Array(wrappedKeyBytes),
      };
    }),
    unwrapKey: vi.fn(async (kek, envelope) => {
      captures.unwrapKek = new Uint8Array(kek);
      captures.unwrapWrappedKey = new Uint8Array(envelope.wrappedKey);

      return new Uint8Array(vaultKeyBytes);
    }),
  };
}

describe("vault crypto metadata", () => {
  it("detects initialized and uninitialized vaults", () => {
    expect(isVaultCryptoInitialized(baseVault)).toBe(false);
    expect(isVaultCryptoInitialized(initializedVault)).toBe(true);
  });

  it("returns decoded metadata for initialized vaults", () => {
    const metadata = requireVaultCryptoMetadata(initializedVault);

    expect(metadata.cryptoVersion).toBe(currentVaultCryptoVersion);
    expect(metadata.kdfVersion).toBe(currentVaultKdfVersion);
    expect(Array.from(metadata.salt)).toEqual(Array.from(saltBytes));
    expect(Array.from(metadata.wrappedKey)).toEqual(
      Array.from(wrappedKeyBytes),
    );
  });

  it("rejects missing metadata", () => {
    expect(() => requireVaultCryptoMetadata(baseVault)).toThrow(ApiError);
  });
});

describe("setupVaultCrypto", () => {
  it("sets up vault crypto metadata without sending passphrases or raw keys", async () => {
    const captures: ProviderCaptures = {};
    const provider = createProvider(captures);
    const requestCalls: Array<[string, ApiRequestOptions | undefined]> = [];
    const requestMock: AuthenticatedRequest = async <T>(
      path: string,
      options?: ApiRequestOptions,
    ): Promise<T> => {
      requestCalls.push([path, options]);

      return {
        vault: initializedVault,
      } as T;
    };

    const result = await setupVaultCrypto({
      provider,
      request: requestMock,
      vaultId: "vault-123",
      passphrase: "vault passphrase",
      randomBytes: (target) => {
        target.set(saltBytes);
      },
    });

    expect(result.vault).toEqual(initializedVault);
    expect(Array.from(result.vaultKey)).toEqual(Array.from(vaultKeyBytes));

    const [path, options] = requestCalls[0];

    expect(path).toBe("/v1/vaults/vault-123/crypto");
    expect(options?.method).toBe("PUT");
    expect(options?.json).toEqual({
      cryptoVersion: currentVaultCryptoVersion,
      kdfVersion: currentVaultKdfVersion,
      salt: bytesToBase64(saltBytes),
      wrappedKey: bytesToBase64(wrappedKeyBytes),
    });
    expect(JSON.stringify(options?.json)).not.toContain("vault passphrase");
    expect(JSON.stringify(options?.json)).not.toContain("3,4,5");

    const [, params] = vi.mocked(provider.deriveKey).mock.calls[0];
    expect(new TextDecoder().decode(captures.derivePassphrase)).toBe(
      "vault passphrase",
    );
    expect(params).toEqual({
      version: CRYPTO_ENVELOPE_VERSION,
      algorithm: "Argon2id",
      salt: saltBytes,
    });
    expect(Array.from(captures.deriveSalt ?? [])).toEqual(
      Array.from(saltBytes),
    );

    expect(Array.from(captures.wrapVaultKey ?? [])).toEqual(
      Array.from(vaultKeyBytes),
    );
  });
});

describe("unlockVaultCrypto", () => {
  it("derives the KEK and unwraps the vault key", async () => {
    const captures: ProviderCaptures = {};
    const provider = createProvider(captures);

    const vaultKey = await unlockVaultCrypto({
      provider,
      vault: initializedVault,
      passphrase: "vault passphrase",
    });

    expect(Array.from(vaultKey)).toEqual(Array.from(vaultKeyBytes));

    const [, params] = vi.mocked(provider.deriveKey).mock.calls[0];
    expect(new TextDecoder().decode(captures.derivePassphrase)).toBe(
      "vault passphrase",
    );
    expect(params).toEqual({
      version: CRYPTO_ENVELOPE_VERSION,
      algorithm: "Argon2id",
      salt: saltBytes,
    });
    expect(Array.from(captures.deriveSalt ?? [])).toEqual(
      Array.from(saltBytes),
    );

    expect(provider.unwrapKey).toHaveBeenCalledTimes(1);
    const [, envelope] = vi.mocked(provider.unwrapKey).mock.calls[0];
    expect(envelope.version).toBe(CRYPTO_ENVELOPE_VERSION);
    expect(envelope.algorithm).toBe("AES-256-GCM");
    expect(Array.from(captures.unwrapKek ?? [])).toEqual(Array.from(kekBytes));
    expect(Array.from(captures.unwrapWrappedKey ?? [])).toEqual(
      Array.from(wrappedKeyBytes),
    );
  });

  it("rejects malformed stored metadata", async () => {
    const provider = createProvider();

    await expect(
      unlockVaultCrypto({
        provider,
        vault: {
          ...initializedVault,
          wrappedKey: "not base64",
        },
        passphrase: "vault passphrase",
      }),
    ).rejects.toThrow(ApiError);
  });
});
