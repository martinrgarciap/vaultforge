use core::fmt;

/// Errors from the vault crypto core.
///
/// SECURITY: no variant carries the passphrase, key, plaintext, or any
/// secret-bearing input — only safe, static descriptions. These are the
/// errors that will cross into JS (via the wasm-bindgen layer in 16D),
/// so they must never leak sensitive data.
#[derive(Debug, PartialEq, Eq)]
pub enum CryptoError {
    /// A provided key was not the required length (keys must be 32 bytes).
    InvalidKeyLength,

    /// The passphrase was empty.
    EmptyPassphrase,

    /// The salt was too short (minimum 16 bytes).
    SaltTooShort,

    /// The input to decrypt/unwrap was too short to contain a nonce.
    InputTooShort,

    /// Encryption failed internally.
    EncryptionFailed,

    /// Decryption failed: wrong key, or the data was tampered with.
    /// (AES-GCM does not distinguish these — by design.)
    DecryptionFailed,

    /// Key derivation (Argon2id) failed internally.
    KeyDerivationFailed,
}

impl fmt::Display for CryptoError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        let msg = match self {
            CryptoError::InvalidKeyLength => "key must be exactly 32 bytes",
            CryptoError::EmptyPassphrase => "passphrase must not be empty",
            CryptoError::SaltTooShort => "salt must be at least 16 bytes",
            CryptoError::InputTooShort => "input too short",
            CryptoError::EncryptionFailed => "encryption failed",
            CryptoError::DecryptionFailed => "decryption failed",
            CryptoError::KeyDerivationFailed => "key derivation failed",
        };
        write!(f, "{msg}")
    }
}

// std::error::Error works here because we build with std for native tests.
// (In pure-wasm no_std contexts this could be gated, but our crate uses std.)
impl std::error::Error for CryptoError {}
