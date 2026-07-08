import { describe, expect, it, vi } from "vitest";

import {
  createWasmCryptoProvider,
  type WasmCryptoModule,
} from "./wasmCryptoProvider";
import {
  CRYPTO_ENVELOPE_VERSION,
  type ItemCryptoEnvelope,
  type VaultKeyDerivationParams,
  type WrappedKeyEnvelope,
} from "./cryptoTypes";

function fakeWasm(): WasmCryptoModule {
  return {
    decrypt: (_key, blob) => blob.slice(1),
    derive_key: (_passphrase, salt) => new Uint8Array(32).fill(salt[0] ?? 0),
    encrypt: (_key, plaintext) => {
      const out = new Uint8Array(plaintext.length + 1);
      out[0] = 7;
      out.set(plaintext, 1);
      return out;
    },
    generate_key: () => new Uint8Array(32).fill(9),
    unwrap_key: (_kek, wrapped) => wrapped.slice(1),
    wrap_key: (_kek, vaultKey) => {
      const out = new Uint8Array(vaultKey.length + 1);
      out[0] = 8;
      out.set(vaultKey, 1);
      return out;
    },
  };
}

describe("WasmCryptoProvider", () => {
  it("loads the wasm module only once", async () => {
    const loadModule = vi.fn(async () => fakeWasm());
    const provider = createWasmCryptoProvider(loadModule);

    await provider.initialize();
    await provider.generateVaultKey();
    await provider.generateVaultKey();

    expect(loadModule).toHaveBeenCalledTimes(1);
  });

  it("generates a 32-byte vault key", async () => {
    const provider = createWasmCryptoProvider(async () => fakeWasm());

    await expect(provider.generateVaultKey()).resolves.toHaveLength(32);
  });

  it("derives a 32-byte key from valid Argon2id params", async () => {
    const provider = createWasmCryptoProvider(async () => fakeWasm());
    const params: VaultKeyDerivationParams = {
      version: CRYPTO_ENVELOPE_VERSION,
      algorithm: "Argon2id",
      salt: new Uint8Array(16).fill(3),
    };

    const key = await provider.deriveKey(new Uint8Array([1, 2, 3]), params);

    expect(key).toHaveLength(32);
    expect(key[0]).toBe(3);
  });

  it("rejects unsupported key derivation params version", async () => {
    const provider = createWasmCryptoProvider(async () => fakeWasm());
    const params: VaultKeyDerivationParams = {
      version: 2 as typeof CRYPTO_ENVELOPE_VERSION,
      algorithm: "Argon2id",
      salt: new Uint8Array(16),
    };

    await expect(
      provider.deriveKey(new Uint8Array([1]), params),
    ).rejects.toThrow("unsupported key derivation params version");
  });

  it("rejects short salts before calling wasm", async () => {
    const loadModule = vi.fn(async () => fakeWasm());
    const provider = createWasmCryptoProvider(loadModule);
    const params: VaultKeyDerivationParams = {
      version: CRYPTO_ENVELOPE_VERSION,
      algorithm: "Argon2id",
      salt: new Uint8Array(8),
    };

    await expect(
      provider.deriveKey(new Uint8Array([1]), params),
    ).rejects.toThrow("salt must be at least 16 bytes");
    expect(loadModule).not.toHaveBeenCalled();
  });

  it("encrypts and decrypts item envelopes", async () => {
    const provider = createWasmCryptoProvider(async () => fakeWasm());
    const key = new Uint8Array(32);
    const plaintext = new Uint8Array([1, 2, 3]);

    const envelope = await provider.encryptItem(key, plaintext);
    const decrypted = await provider.decryptItem(key, envelope);

    expect(envelope).toEqual({
      version: CRYPTO_ENVELOPE_VERSION,
      algorithm: "AES-256-GCM",
      blob: new Uint8Array([7, 1, 2, 3]),
    });
    expect(decrypted).toEqual(plaintext);
  });

  it("rejects unsupported item envelope version", async () => {
    const provider = createWasmCryptoProvider(async () => fakeWasm());
    const envelope: ItemCryptoEnvelope = {
      version: 2 as typeof CRYPTO_ENVELOPE_VERSION,
      algorithm: "AES-256-GCM",
      blob: new Uint8Array([1]),
    };

    await expect(
      provider.decryptItem(new Uint8Array(32), envelope),
    ).rejects.toThrow("unsupported item crypto envelope version");
  });

  it("wraps and unwraps vault key envelopes", async () => {
    const provider = createWasmCryptoProvider(async () => fakeWasm());
    const kek = new Uint8Array(32);
    const vaultKey = new Uint8Array(32).fill(5);

    const envelope = await provider.wrapKey(kek, vaultKey);
    const unwrapped = await provider.unwrapKey(kek, envelope);

    expect(envelope.version).toBe(CRYPTO_ENVELOPE_VERSION);
    expect(envelope.algorithm).toBe("AES-256-GCM");
    expect(unwrapped).toEqual(vaultKey);
  });

  it("rejects unsupported wrapped key envelope version", async () => {
    const provider = createWasmCryptoProvider(async () => fakeWasm());
    const envelope: WrappedKeyEnvelope = {
      version: 2 as typeof CRYPTO_ENVELOPE_VERSION,
      algorithm: "AES-256-GCM",
      wrappedKey: new Uint8Array([1]),
    };

    await expect(
      provider.unwrapKey(new Uint8Array(32), envelope),
    ).rejects.toThrow("unsupported wrapped key envelope version");
  });

  it("rejects malformed generated key lengths", async () => {
    const provider = createWasmCryptoProvider(async () => ({
      ...fakeWasm(),
      generate_key: () => new Uint8Array(16),
    }));

    await expect(provider.generateVaultKey()).rejects.toThrow(
      "vault key must be 32 bytes",
    );
  });
});
