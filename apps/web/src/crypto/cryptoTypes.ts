export const CRYPTO_ENVELOPE_VERSION = 1;

export const KEY_BYTES = 32;
export const MIN_SALT_BYTES = 16;

export type CryptoEnvelopeVersion = typeof CRYPTO_ENVELOPE_VERSION;

export interface ItemCryptoEnvelope {
  version: CryptoEnvelopeVersion;
  algorithm: "AES-256-GCM";
  blob: Uint8Array;
}

export interface WrappedKeyEnvelope {
  version: CryptoEnvelopeVersion;
  algorithm: "AES-256-GCM";
  wrappedKey: Uint8Array;
}

export interface VaultKeyDerivationParams {
  version: CryptoEnvelopeVersion;
  algorithm: "Argon2id";
  salt: Uint8Array;
}
