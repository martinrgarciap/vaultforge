use aes_gcm::{
    aead::{rand_core::RngCore, Aead, KeyInit, OsRng},
    Aes256Gcm, Key, Nonce,
};
use argon2::Argon2;

use crate::error::CryptoError;

const NONCE_LEN: usize = 12;
const KEY_LEN: usize = 32;
const MIN_SALT_LEN: usize = 16;

/// Derive a 32-byte key from a passphrase + salt using Argon2id.
/// Reproducible: same passphrase + salt → same key.
pub fn derive_key_impl(passphrase: &[u8], salt: &[u8]) -> Result<[u8; 32], CryptoError> {
    if passphrase.is_empty() {
        return Err(CryptoError::EmptyPassphrase);
    }
    if salt.len() < MIN_SALT_LEN {
        return Err(CryptoError::SaltTooShort);
    }

    let argon2 = Argon2::default(); // Argon2id, secure defaults
    let mut key = [0u8; KEY_LEN];
    argon2
        .hash_password_into(passphrase, salt, &mut key)
        .map_err(|_| CryptoError::KeyDerivationFailed)?;
    Ok(key)
}

/// Generate a fresh random 32-byte key (e.g. a vault master key).
pub fn generate_key_impl() -> Result<[u8; 32], CryptoError> {
    let mut key = [0u8; KEY_LEN];
    OsRng.fill_bytes(&mut key);
    Ok(key)
}

/// Encrypt plaintext with a 32-byte key. Output: [nonce][ciphertext+tag].
pub fn encrypt_impl(key: &[u8], plaintext: &[u8]) -> Result<Vec<u8>, CryptoError> {
    let key: [u8; KEY_LEN] = key.try_into().map_err(|_| CryptoError::InvalidKeyLength)?;
    let cipher = Aes256Gcm::new(Key::<Aes256Gcm>::from_slice(&key));

    let mut nonce_bytes = [0u8; NONCE_LEN];
    OsRng.fill_bytes(&mut nonce_bytes);
    let nonce = Nonce::from_slice(&nonce_bytes);

    let ciphertext = cipher
        .encrypt(nonce, plaintext)
        .map_err(|_| CryptoError::EncryptionFailed)?;

    let mut out = Vec::with_capacity(NONCE_LEN + ciphertext.len());
    out.extend_from_slice(&nonce_bytes);
    out.extend_from_slice(&ciphertext);
    Ok(out)
}

/// Decrypt a [nonce][ciphertext+tag] blob with a 32-byte key.
pub fn decrypt_impl(key: &[u8], blob: &[u8]) -> Result<Vec<u8>, CryptoError> {
    let key: [u8; KEY_LEN] = key.try_into().map_err(|_| CryptoError::InvalidKeyLength)?;
    if blob.len() < NONCE_LEN {
        return Err(CryptoError::InputTooShort);
    }

    let (nonce_bytes, ciphertext) = blob.split_at(NONCE_LEN);
    let nonce = Nonce::from_slice(nonce_bytes);
    let cipher = Aes256Gcm::new(Key::<Aes256Gcm>::from_slice(&key));

    cipher
        .decrypt(nonce, ciphertext)
        .map_err(|_| CryptoError::DecryptionFailed)
}

/// Wrap (encrypt) a 32-byte master key with a KEK.
pub fn wrap_key_impl(kek: &[u8], vault_key: &[u8]) -> Result<Vec<u8>, CryptoError> {
    if vault_key.len() != KEY_LEN {
        return Err(CryptoError::InvalidKeyLength);
    }
    encrypt_impl(kek, vault_key)
}

/// Unwrap (decrypt) a wrapped master key with a KEK.
pub fn unwrap_key_impl(kek: &[u8], wrapped: &[u8]) -> Result<Vec<u8>, CryptoError> {
    let key = decrypt_impl(kek, wrapped)?;
    if key.len() != KEY_LEN {
        return Err(CryptoError::InvalidKeyLength);
    }
    Ok(key)
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::error::CryptoError;

    fn test_key() -> [u8; 32] {
        [7u8; 32]
    }

    // --- key generation ---
    #[test]
    fn generate_key_returns_32_bytes() {
        let k = generate_key_impl().unwrap();
        assert_eq!(k.len(), 32);
    }

    #[test]
    fn generate_key_differs_each_call() {
        let a = generate_key_impl().unwrap();
        let b = generate_key_impl().unwrap();
        assert_ne!(a, b);
    }

    // --- key derivation ---
    #[test]
    fn derive_key_is_reproducible() {
        let salt = [3u8; 16];
        let k1 = derive_key_impl(b"passphrase", &salt).unwrap();
        let k2 = derive_key_impl(b"passphrase", &salt).unwrap();
        assert_eq!(k1, k2);
        assert_eq!(k1.len(), 32);
    }

    #[test]
    fn derive_key_changes_with_salt() {
        let k1 = derive_key_impl(b"pass", &[1u8; 16]).unwrap();
        let k2 = derive_key_impl(b"pass", &[2u8; 16]).unwrap();
        assert_ne!(k1, k2);
    }

    #[test]
    fn derive_key_empty_passphrase_rejected() {
        assert_eq!(
            derive_key_impl(b"", &[0u8; 16]),
            Err(CryptoError::EmptyPassphrase)
        );
    }

    #[test]
    fn derive_key_short_salt_rejected() {
        assert_eq!(
            derive_key_impl(b"pass", &[0u8; 8]),
            Err(CryptoError::SaltTooShort)
        );
    }

    // --- encrypt / decrypt ---
    #[test]
    fn encrypt_decrypt_round_trips() {
        let key = test_key();
        let blob = encrypt_impl(&key, b"secret data").unwrap();
        assert_eq!(decrypt_impl(&key, &blob).unwrap(), b"secret data");
    }

    #[test]
    fn same_plaintext_encrypts_differently() {
        // Fresh nonce each time → different ciphertext.
        let key = test_key();
        let a = encrypt_impl(&key, b"same").unwrap();
        let b = encrypt_impl(&key, b"same").unwrap();
        assert_ne!(a, b);
    }

    #[test]
    fn wrong_key_fails_decrypt() {
        let blob = encrypt_impl(&test_key(), b"secret").unwrap();
        assert_eq!(
            decrypt_impl(&[9u8; 32], &blob),
            Err(CryptoError::DecryptionFailed)
        );
    }

    #[test]
    fn tampered_ciphertext_fails_decrypt() {
        let key = test_key();
        let mut blob = encrypt_impl(&key, b"secret").unwrap();
        let last = blob.len() - 1;
        blob[last] ^= 0x01;
        assert_eq!(
            decrypt_impl(&key, &blob),
            Err(CryptoError::DecryptionFailed)
        );
    }

    #[test]
    fn invalid_key_length_rejected() {
        assert_eq!(
            encrypt_impl(&[0u8; 16], b"x"),
            Err(CryptoError::InvalidKeyLength)
        );
    }

    #[test]
    fn short_blob_fails_decrypt() {
        assert_eq!(
            decrypt_impl(&test_key(), &[0u8; 5]),
            Err(CryptoError::InputTooShort)
        );
    }

    // --- wrap / unwrap ---
    #[test]
    fn wrap_unwrap_round_trips() {
        let kek = test_key();
        let master = [5u8; 32];
        let wrapped = wrap_key_impl(&kek, &master).unwrap();
        assert_eq!(unwrap_key_impl(&kek, &wrapped).unwrap(), master);
    }

    #[test]
    fn unwrap_wrong_kek_fails() {
        let wrapped = wrap_key_impl(&test_key(), &[5u8; 32]).unwrap();
        assert_eq!(
            unwrap_key_impl(&[9u8; 32], &wrapped),
            Err(CryptoError::DecryptionFailed)
        );
    }

    #[test]
    fn wrap_rejects_wrong_size_key() {
        assert_eq!(
            wrap_key_impl(&test_key(), &[5u8; 16]),
            Err(CryptoError::InvalidKeyLength)
        );
    }
}
