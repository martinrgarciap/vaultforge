use argon2::password_hash::rand_core::OsRng;
use argon2::{
    password_hash::{PasswordHash, PasswordHasher, PasswordVerifier, SaltString},
    Argon2,
};

use crate::error::HashError;

/// Hash an account password with Argon2id.
///
/// Validates length bounds, generates a fresh random salt, and returns a
/// PHC-encoded string safe to store. Never logs or echoes the password.
pub fn hash_password(password: &str, max_password_len: usize) -> Result<String, HashError> {
    validate_input(password, "password", max_password_len)?;

    let salt = SaltString::generate(&mut OsRng);
    let argon2 = Argon2::default(); // Argon2id, secure defaults

    argon2
        .hash_password(password.as_bytes(), &salt)
        .map(|hash| hash.to_string())
        .map_err(|_| HashError::Internal) // never leak argon2's detail
}

/// Verify an account password against a stored PHC hash.
///
/// Returns:
/// - `Ok(true)`  — password matches
/// - `Ok(false)` — password does NOT match (a NORMAL outcome, not an error)
/// - `Err(..)`   — invalid input or a malformed stored hash
pub fn verify_password(
    password: &str,
    phc_hash: &str,
    max_password_len: usize,
    max_hash_len: usize,
) -> Result<bool, HashError> {
    validate_input(password, "password", max_password_len)?;
    validate_input(phc_hash, "hash", max_hash_len)?;

    // Parse the PHC string; a parse failure is a malformed hash (client input).
    let parsed = PasswordHash::new(phc_hash).map_err(|_| HashError::MalformedHash)?;

    // Verify. A mismatch is Ok(false); only unexpected errors bubble up.
    match Argon2::default().verify_password(password.as_bytes(), &parsed) {
        Ok(()) => Ok(true),
        Err(argon2::password_hash::Error::Password) => Ok(false),
        Err(_) => Err(HashError::MalformedHash),
    }
}

/// Validate any secret-bearing text input: non-empty, within max byte length.
fn validate_input(value: &str, field: &'static str, max_len: usize) -> Result<(), HashError> {
    if value.is_empty() {
        return Err(HashError::Empty { field });
    }
    let len = value.len();
    if len > max_len {
        return Err(HashError::TooLong {
            field,
            actual: len,
            max: max_len,
        });
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;

    const MAX_PW: usize = 1024;
    const MAX_HASH: usize = 512;

    #[test]
    fn hash_then_verify_succeeds() {
        let phc = hash_password("correct horse", MAX_PW).unwrap();
        assert_eq!(
            verify_password("correct horse", &phc, MAX_PW, MAX_HASH).unwrap(),
            true
        );
    }

    #[test]
    fn wrong_password_returns_false_not_error() {
        let phc = hash_password("real-password", MAX_PW).unwrap();
        assert_eq!(
            verify_password("wrong-password", &phc, MAX_PW, MAX_HASH).unwrap(),
            false
        );
    }

    #[test]
    fn same_password_produces_different_hashes() {
        let a = hash_password("same", MAX_PW).unwrap();
        let b = hash_password("same", MAX_PW).unwrap();
        assert_ne!(a, b);
        assert!(verify_password("same", &a, MAX_PW, MAX_HASH).unwrap());
        assert!(verify_password("same", &b, MAX_PW, MAX_HASH).unwrap());
    }

    #[test]
    fn empty_password_is_rejected() {
        assert_eq!(
            hash_password("", MAX_PW),
            Err(HashError::Empty { field: "password" })
        );
    }

    #[test]
    fn oversized_password_is_rejected() {
        let big = "a".repeat(MAX_PW + 1);
        assert!(matches!(
            hash_password(&big, MAX_PW),
            Err(HashError::TooLong {
                field: "password",
                ..
            })
        ));
    }

    #[test]
    fn password_exactly_at_limit_is_accepted() {
        let exact = "a".repeat(MAX_PW);
        assert!(hash_password(&exact, MAX_PW).is_ok());
    }

    #[test]
    fn verify_empty_password_is_rejected() {
        let phc = hash_password("x", MAX_PW).unwrap();
        assert_eq!(
            verify_password("", &phc, MAX_PW, MAX_HASH),
            Err(HashError::Empty { field: "password" })
        );
    }

    #[test]
    fn verify_empty_hash_is_rejected() {
        assert_eq!(
            verify_password("x", "", MAX_PW, MAX_HASH),
            Err(HashError::Empty { field: "hash" })
        );
    }

    #[test]
    fn verify_oversized_hash_is_rejected() {
        let big = "a".repeat(MAX_HASH + 1);
        assert!(matches!(
            verify_password("x", &big, MAX_PW, MAX_HASH),
            Err(HashError::TooLong { field: "hash", .. })
        ));
    }

    #[test]
    fn malformed_hash_is_rejected() {
        assert_eq!(
            verify_password("x", "not-a-phc-string", MAX_PW, MAX_HASH),
            Err(HashError::MalformedHash)
        );
    }
}
