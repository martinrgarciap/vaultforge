import type {
  ItemCryptoEnvelope,
  VaultKeyDerivationParams,
  WrappedKeyEnvelope,
} from "./cryptoTypes";

export interface CryptoProvider {
  initialize(): Promise<void>;

  generateVaultKey(): Promise<Uint8Array>;

  deriveKey(
    passphrase: Uint8Array,
    params: VaultKeyDerivationParams,
  ): Promise<Uint8Array>;

  encryptItem(
    key: Uint8Array,
    plaintext: Uint8Array,
  ): Promise<ItemCryptoEnvelope>;

  decryptItem(
    key: Uint8Array,
    envelope: ItemCryptoEnvelope,
  ): Promise<Uint8Array>;

  wrapKey(kek: Uint8Array, vaultKey: Uint8Array): Promise<WrappedKeyEnvelope>;

  unwrapKey(kek: Uint8Array, envelope: WrappedKeyEnvelope): Promise<Uint8Array>;
}
