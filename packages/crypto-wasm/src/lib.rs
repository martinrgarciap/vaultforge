//! VaultForge browser-side vault cryptography (WASM).
//!
//! Scope: vault passphrase key derivation, key wrapping, and vault item
//! encryption/decryption — all browser-side. The vault master passphrase and
//! unwrapped keys NEVER leave the browser or reach the Go API.
//!

pub mod crypto;
pub mod error;
pub mod wasm;

pub use crypto::{
    decrypt_impl, derive_key_impl, encrypt_impl, generate_key_impl, unwrap_key_impl, wrap_key_impl,
};
pub use error::CryptoError;
