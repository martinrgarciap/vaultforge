import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

import init, {
  decrypt,
  derive_key,
  encrypt,
  generate_key,
  unwrap_key,
  wrap_key,
} from "../../../../packages/crypto-wasm/pkg/crypto_wasm.js";

import { CRYPTO_ENVELOPE_VERSION } from "../../src/crypto/cryptoTypes";
import {
  createWasmCryptoProvider,
  type WasmCryptoModule,
} from "../../src/crypto/wasmCryptoProvider";

const encoder = new TextEncoder();
const decoder = new TextDecoder();

let modulePromise: Promise<WasmCryptoModule> | null = null;

function loadCryptoWasmFromDisk(): Promise<WasmCryptoModule> {
  modulePromise ??= initializeCryptoWasmFromDisk();
  return modulePromise;
}

async function initializeCryptoWasmFromDisk(): Promise<WasmCryptoModule> {
  const wasmPath = fileURLToPath(
    new URL(
      "../../../../packages/crypto-wasm/pkg/crypto_wasm_bg.wasm",
      import.meta.url,
    ),
  );
  const wasmBytes = await readFile(wasmPath);

  await init({ module_or_path: wasmBytes });

  return {
    decrypt,
    derive_key,
    encrypt,
    generate_key,
    unwrap_key,
    wrap_key,
  };
}

describe("real crypto WASM integration", () => {
  it("loads the real Rust WASM module and encrypts/decrypts an item", async () => {
    const provider = createWasmCryptoProvider(loadCryptoWasmFromDisk);
    const key = await provider.generateVaultKey();
    const plaintext = encoder.encode("synthetic vault item secret");

    const envelope = await provider.encryptItem(key, plaintext);
    const decrypted = await provider.decryptItem(key, envelope);

    expect(envelope.version).toBe(CRYPTO_ENVELOPE_VERSION);
    expect(envelope.algorithm).toBe("AES-256-GCM");
    expect(envelope.blob.length).toBeGreaterThan(plaintext.length);
    expect(decoder.decode(decrypted)).toBe("synthetic vault item secret");
  });

  it("loads the real Rust WASM module and wraps/unwraps a vault key", async () => {
    const provider = createWasmCryptoProvider(loadCryptoWasmFromDisk);
    const kek = await provider.deriveKey(
      encoder.encode("synthetic passphrase"),
      {
        version: CRYPTO_ENVELOPE_VERSION,
        algorithm: "Argon2id",
        salt: new Uint8Array(16).fill(4),
      },
    );
    const vaultKey = await provider.generateVaultKey();

    const wrapped = await provider.wrapKey(kek, vaultKey);
    const unwrapped = await provider.unwrapKey(kek, wrapped);

    expect(wrapped.version).toBe(CRYPTO_ENVELOPE_VERSION);
    expect(wrapped.algorithm).toBe("AES-256-GCM");
    expect(unwrapped).toEqual(vaultKey);
  });

  it("rejects decrypting real Rust WASM ciphertext with the wrong key", async () => {
    const provider = createWasmCryptoProvider(loadCryptoWasmFromDisk);
    const key = await provider.generateVaultKey();
    const wrongKey = await provider.generateVaultKey();

    const envelope = await provider.encryptItem(
      key,
      encoder.encode("synthetic secret"),
    );

    await expect(provider.decryptItem(wrongKey, envelope)).rejects.toThrow(
      "decryption failed",
    );
  });
});
