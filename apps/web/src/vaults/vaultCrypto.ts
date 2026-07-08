import { ApiError } from "../api/ApiError";
import type { ApiRequestOptions } from "../api/types";
import type { CryptoProvider } from "../crypto/CryptoProvider";
import {
  CRYPTO_ENVELOPE_VERSION,
  type VaultKeyDerivationParams,
  type WrappedKeyEnvelope,
} from "../crypto/cryptoTypes";
import { base64ToBytes, bytesToBase64 } from "../crypto/encoding";
import {
  currentVaultCryptoVersion,
  currentVaultKdfVersion,
  parseVaultResponse,
  type InitializeVaultCryptoRequest,
  type Vault,
  type VaultResponse,
} from "./contracts";

export const vaultCryptoSaltBytes = 32;
const minimumWrappedVaultKeyBytes = 60;

export type AuthenticatedRequest = <T>(
  path: string,
  options?: ApiRequestOptions,
) => Promise<T>;

export type RandomBytes = (target: Uint8Array) => void;

export interface VaultCryptoMetadata {
  cryptoVersion: typeof currentVaultCryptoVersion;
  kdfVersion: typeof currentVaultKdfVersion;
  salt: Uint8Array;
  wrappedKey: Uint8Array;
}

export interface SetupVaultCryptoInput {
  provider: CryptoProvider;
  request: AuthenticatedRequest;
  vaultId: string;
  passphrase: string;
  randomBytes?: RandomBytes;
}

export interface UnlockVaultCryptoInput {
  provider: CryptoProvider;
  vault: Vault;
  passphrase: string;
}

export interface VaultCryptoSession {
  vault: Vault;
  vaultKey: Uint8Array;
}

const textEncoder = new TextEncoder();

export function isVaultCryptoInitialized(vault: Vault): boolean {
  return (
    vault.cryptoVersion !== undefined &&
    vault.kdfVersion !== undefined &&
    vault.salt !== undefined &&
    vault.wrappedKey !== undefined
  );
}

export function requireVaultCryptoMetadata(vault: Vault): VaultCryptoMetadata {
  if (!isVaultCryptoInitialized(vault)) {
    throw ApiError.invalidResponse();
  }

  if (
    vault.cryptoVersion !== currentVaultCryptoVersion ||
    vault.kdfVersion !== currentVaultKdfVersion ||
    vault.salt === undefined ||
    vault.wrappedKey === undefined
  ) {
    throw ApiError.invalidResponse();
  }

  return {
    cryptoVersion: currentVaultCryptoVersion,
    kdfVersion: currentVaultKdfVersion,
    salt: decodeVaultCryptoBytes(vault.salt),
    wrappedKey: decodeVaultCryptoBytes(vault.wrappedKey),
  };
}

export function generateVaultCryptoSalt(
  randomBytes: RandomBytes = browserRandomBytes,
): Uint8Array {
  const salt = new Uint8Array(vaultCryptoSaltBytes);
  randomBytes(salt);

  return salt;
}

export async function setupVaultCrypto({
  provider,
  request,
  vaultId,
  passphrase,
  randomBytes,
}: SetupVaultCryptoInput): Promise<VaultCryptoSession> {
  const salt = generateVaultCryptoSalt(randomBytes);
  const passphraseBytes = encodePassphrase(passphrase);
  const kek = await deriveVaultKek(provider, passphraseBytes, salt);
  const vaultKey = await provider.generateVaultKey();

  try {
    const wrappedKeyEnvelope = await provider.wrapKey(kek, vaultKey);
    assertSupportedWrappedKeyEnvelope(wrappedKeyEnvelope);

    const body = {
      cryptoVersion: currentVaultCryptoVersion,
      kdfVersion: currentVaultKdfVersion,
      salt: bytesToBase64(salt),
      wrappedKey: bytesToBase64(wrappedKeyEnvelope.wrappedKey),
    } satisfies InitializeVaultCryptoRequest;

    const response = parseVaultResponse(
      await request<VaultResponse>(vaultCryptoPath(vaultId), {
        method: "PUT",
        json: body,
      }),
    );

    return {
      vault: response.vault,
      vaultKey: new Uint8Array(vaultKey),
    };
  } finally {
    zeroize(passphraseBytes);
    zeroize(kek);
    zeroize(vaultKey);
  }
}

export async function unlockVaultCrypto({
  provider,
  vault,
  passphrase,
}: UnlockVaultCryptoInput): Promise<Uint8Array> {
  const metadata = requireVaultCryptoMetadata(vault);
  const passphraseBytes = encodePassphrase(passphrase);
  const kek = await deriveVaultKek(provider, passphraseBytes, metadata.salt);
  const wrappedKey = new Uint8Array(metadata.wrappedKey);
  let vaultKey: Uint8Array | undefined;

  try {
    const envelope: WrappedKeyEnvelope = {
      version: CRYPTO_ENVELOPE_VERSION,
      algorithm: "AES-256-GCM",
      wrappedKey,
    };

    assertSupportedWrappedKeyEnvelope(envelope);
    vaultKey = await provider.unwrapKey(kek, envelope);

    return new Uint8Array(vaultKey);
  } finally {
    zeroize(passphraseBytes);
    zeroize(wrappedKey);
    zeroize(kek);

    if (vaultKey !== undefined) {
      zeroize(vaultKey);
    }
  }
}

function browserRandomBytes(target: Uint8Array): void {
  const randomValues = crypto.getRandomValues(new Uint8Array(target.length));

  target.set(randomValues);
}

function encodePassphrase(passphrase: string): Uint8Array {
  if (passphrase.length === 0) {
    throw new TypeError("Vault passphrase is required.");
  }

  return textEncoder.encode(passphrase);
}

function deriveVaultKek(
  provider: CryptoProvider,
  passphraseBytes: Uint8Array,
  salt: Uint8Array,
): Promise<Uint8Array> {
  const params: VaultKeyDerivationParams = {
    version: CRYPTO_ENVELOPE_VERSION,
    algorithm: "Argon2id",
    salt,
  };

  return provider.deriveKey(passphraseBytes, params);
}

function decodeVaultCryptoBytes(value: string): Uint8Array {
  try {
    return base64ToBytes(value, "vault crypto metadata");
  } catch {
    throw ApiError.invalidResponse();
  }
}

function assertSupportedWrappedKeyEnvelope(envelope: WrappedKeyEnvelope): void {
  if (
    envelope.version !== CRYPTO_ENVELOPE_VERSION ||
    envelope.algorithm !== "AES-256-GCM" ||
    envelope.wrappedKey.length < minimumWrappedVaultKeyBytes
  ) {
    throw ApiError.invalidResponse();
  }
}

function vaultCryptoPath(vaultId: string): string {
  if (vaultId.trim() === "") {
    throw new TypeError("Vault ID is required.");
  }

  return `/v1/vaults/${encodeURIComponent(vaultId)}/crypto`;
}

function zeroize(bytes: Uint8Array): void {
  bytes.fill(0);
}
