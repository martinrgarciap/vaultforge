// Thin wasm-bindgen wrappers exposing the crypto core to browser JavaScript.
//
// These wrappers contain NO crypto logic — each simply calls the corresponding
// pure `_impl` function (in crypto.rs) and converts a CryptoError into a JsValue
// so it surfaces as a thrown JS error. All real logic and all tests live in the
// native `_impl` functions, which never depend on wasm-bindgen.

use wasm_bindgen::prelude::*;

use crate::crypto::{
    decrypt_impl, derive_key_impl, encrypt_impl, generate_key_impl, unwrap_key_impl,
    wrap_key_impl,
};
use crate::error::CryptoError;

// Convert our typed error into a JS-visible error value.
// Centralized here so every wrapper converts errors identically.
impl From<CryptoError> for JsValue {
    fn from(err: CryptoError) -> JsValue {
        JsValue::from_str(&err.to_string())
    }
}

/// Derive a 32-byte vault key from a passphrase + salt (Argon2id).
#[wasm_bindgen]
pub fn derive_key(passphrase: &[u8], salt: &[u8]) -> Result<Vec<u8>, JsValue> {
    Ok(derive_key_impl(passphrase, salt)?.to_vec())
}

/// Generate a fresh random 32-byte key.
#[wasm_bindgen]
pub fn generate_key() -> Result<Vec<u8>, JsValue> {
    Ok(generate_key_impl()?.to_vec())
}

/// Encrypt plaintext with a 32-byte key → [nonce][ciphertext+tag].
#[wasm_bindgen]
pub fn encrypt(key: &[u8], plaintext: &[u8]) -> Result<Vec<u8>, JsValue> {
    Ok(encrypt_impl(key, plaintext)?)
}

/// Decrypt a [nonce][ciphertext+tag] blob with a 32-byte key.
#[wasm_bindgen]
pub fn decrypt(key: &[u8], blob: &[u8]) -> Result<Vec<u8>, JsValue> {
    Ok(decrypt_impl(key, blob)?)
}

/// Wrap (encrypt) a 32-byte vault key with a KEK.
#[wasm_bindgen]
pub fn wrap_key(kek: &[u8], vault_key: &[u8]) -> Result<Vec<u8>, JsValue> {
    Ok(wrap_key_impl(kek, vault_key)?)
}

/// Unwrap (decrypt) a wrapped vault key with a KEK.
#[wasm_bindgen]
pub fn unwrap_key(kek: &[u8], wrapped: &[u8]) -> Result<Vec<u8>, JsValue> {
    Ok(unwrap_key_impl(kek, wrapped)?)
}