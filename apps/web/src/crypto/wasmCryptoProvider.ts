import type { CryptoProvider } from "./CryptoProvider";
import {
  CRYPTO_ENVELOPE_VERSION,
  KEY_BYTES,
  MIN_SALT_BYTES,
  type ItemCryptoEnvelope,
  type VaultKeyDerivationParams,
  type WrappedKeyEnvelope,
} from "./cryptoTypes";

export interface WasmCryptoModule {
  decrypt(key: Uint8Array, blob: Uint8Array): Uint8Array;
  derive_key(passphrase: Uint8Array, salt: Uint8Array): Uint8Array;
  encrypt(key: Uint8Array, plaintext: Uint8Array): Uint8Array;
  generate_key(): Uint8Array;
  unwrap_key(kek: Uint8Array, wrapped: Uint8Array): Uint8Array;
  wrap_key(kek: Uint8Array, vaultKey: Uint8Array): Uint8Array;
}

export type WasmCryptoModuleLoader = () => Promise<WasmCryptoModule>;

export class WasmCryptoProvider implements CryptoProvider {
  private modulePromise: Promise<WasmCryptoModule> | null = null;

  constructor(private readonly loadModule: WasmCryptoModuleLoader) {}

  async initialize(): Promise<void> {
    await this.getModule();
  }

  async generateVaultKey(): Promise<Uint8Array> {
    const wasm = await this.getModule();
    const key = wasm.generate_key();
    assertLength(key, KEY_BYTES, "vault key");
    return key;
  }

  async deriveKey(
    passphrase: Uint8Array,
    params: VaultKeyDerivationParams,
  ): Promise<Uint8Array> {
    if (params.version !== CRYPTO_ENVELOPE_VERSION) {
      throw new Error("unsupported key derivation params version");
    }
    if (params.algorithm !== "Argon2id") {
      throw new Error("unsupported key derivation algorithm");
    }
    if (params.salt.length < MIN_SALT_BYTES) {
      throw new Error("salt must be at least 16 bytes");
    }

    const wasm = await this.getModule();
    const key = wasm.derive_key(passphrase, params.salt);
    assertLength(key, KEY_BYTES, "derived key");
    return key;
  }

  async encryptItem(
    key: Uint8Array,
    plaintext: Uint8Array,
  ): Promise<ItemCryptoEnvelope> {
    const wasm = await this.getModule();

    return {
      version: CRYPTO_ENVELOPE_VERSION,
      algorithm: "AES-256-GCM",
      blob: wasm.encrypt(key, plaintext),
    };
  }

  async decryptItem(
    key: Uint8Array,
    envelope: ItemCryptoEnvelope,
  ): Promise<Uint8Array> {
    if (envelope.version !== CRYPTO_ENVELOPE_VERSION) {
      throw new Error("unsupported item crypto envelope version");
    }
    if (envelope.algorithm !== "AES-256-GCM") {
      throw new Error("unsupported item crypto algorithm");
    }

    const wasm = await this.getModule();
    return wasm.decrypt(key, envelope.blob);
  }

  async wrapKey(
    kek: Uint8Array,
    vaultKey: Uint8Array,
  ): Promise<WrappedKeyEnvelope> {
    const wasm = await this.getModule();

    return {
      version: CRYPTO_ENVELOPE_VERSION,
      algorithm: "AES-256-GCM",
      wrappedKey: wasm.wrap_key(kek, vaultKey),
    };
  }

  async unwrapKey(
    kek: Uint8Array,
    envelope: WrappedKeyEnvelope,
  ): Promise<Uint8Array> {
    if (envelope.version !== CRYPTO_ENVELOPE_VERSION) {
      throw new Error("unsupported wrapped key envelope version");
    }
    if (envelope.algorithm !== "AES-256-GCM") {
      throw new Error("unsupported wrapped key algorithm");
    }

    const wasm = await this.getModule();
    const key = wasm.unwrap_key(kek, envelope.wrappedKey);
    assertLength(key, KEY_BYTES, "unwrapped vault key");
    return key;
  }

  private getModule(): Promise<WasmCryptoModule> {
    this.modulePromise ??= this.loadModule();
    return this.modulePromise;
  }
}

export function createWasmCryptoProvider(
  loadModule: WasmCryptoModuleLoader,
): CryptoProvider {
  return new WasmCryptoProvider(loadModule);
}

function assertLength(
  value: Uint8Array,
  expected: number,
  label: string,
): void {
  if (value.length !== expected) {
    throw new Error(`${label} must be ${expected} bytes`);
  }
}
