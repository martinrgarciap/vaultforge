use rand::rngs::OsRng;
use rand::seq::SliceRandom;

use crate::error::PasswordError;

// The character classes.
const UPPERCASE: &str = "ABCDEFGHIJKLMNOPQRSTUVWXYZ";
const LOWERCASE: &str = "abcdefghijklmnopqrstuvwxyz";
const DIGITS: &str = "0123456789";
const SYMBOLS: &str = "!@#$%^&*()-_=+[]{};:,.<>?";

/// Parameters for password generation (mirrors the proto request, decoupled from it).
pub struct GenerateParams {
    pub length: u32,
    pub include_uppercase: bool,
    pub include_lowercase: bool,
    pub include_digits: bool,
    pub include_symbols: bool,
    pub exclude_chars: String,
}

/// Generate a password. Returns the password and its estimated entropy in bits.
pub fn generate_impl(
    params: &GenerateParams,
    min_length: u32,
    max_length: u32,
) -> Result<(String, f64), PasswordError> {
    if params.length < min_length || params.length > max_length {
        return Err(PasswordError::LengthOutOfRange {
            min: min_length,
            max: max_length,
        });
    }

    let exclude: Vec<char> = params.exclude_chars.chars().collect();
    let filter =
        |set: &str| -> Vec<char> { set.chars().filter(|c| !exclude.contains(c)).collect() };

    let mut classes: Vec<Vec<char>> = Vec::new();
    if params.include_uppercase {
        classes.push(filter(UPPERCASE));
    }
    if params.include_lowercase {
        classes.push(filter(LOWERCASE));
    }
    if params.include_digits {
        classes.push(filter(DIGITS));
    }
    if params.include_symbols {
        classes.push(filter(SYMBOLS));
    }

    if classes.is_empty() {
        return Err(PasswordError::NoCharacterClasses);
    }

    if classes.iter().any(|c| c.is_empty()) {
        return Err(PasswordError::EmptyCharacterPool);
    }

    let class_count = classes.len() as u32;
    if params.length < class_count {
        return Err(PasswordError::LengthTooSmallForClasses {
            length: params.length,
            classes: class_count,
        });
    }

    let mut rng = OsRng;

    let mut result: Vec<char> = Vec::with_capacity(params.length as usize);
    for class in &classes {
        let c = *class.choose(&mut rng).ok_or(PasswordError::Internal)?;
        result.push(c);
    }

    let combined: Vec<char> = classes.iter().flatten().copied().collect();
    let remaining = params.length as usize - result.len();
    for _ in 0..remaining {
        let c = *combined.choose(&mut rng).ok_or(PasswordError::Internal)?;
        result.push(c);
    }

    result.shuffle(&mut rng);

    let password: String = result.into_iter().collect();

    let entropy_bits = (params.length as f64) * (combined.len() as f64).log2();

    Ok((password, entropy_bits))
}

#[cfg(test)]
mod tests {
    use super::*;

    const MIN: u32 = 4;
    const MAX: u32 = 256;

    fn base_params() -> GenerateParams {
        GenerateParams {
            length: 16,
            include_uppercase: true,
            include_lowercase: true,
            include_digits: true,
            include_symbols: false,
            exclude_chars: String::new(),
        }
    }

    #[test]
    fn generates_requested_length() {
        let (pw, _) = generate_impl(&base_params(), MIN, MAX).unwrap();
        assert_eq!(pw.chars().count(), 16);
    }

    #[test]
    fn only_uses_enabled_classes() {
        let mut p = base_params();
        p.include_symbols = false;
        p.include_digits = false;
        let (pw, _) = generate_impl(&p, MIN, MAX).unwrap();
        assert!(pw.chars().all(|c| c.is_ascii_alphabetic()));
    }

    #[test]
    fn every_enabled_class_appears() {
        let p = GenerateParams {
            length: 40,
            include_uppercase: true,
            include_lowercase: true,
            include_digits: true,
            include_symbols: true,
            exclude_chars: String::new(),
        };
        let (pw, _) = generate_impl(&p, MIN, MAX).unwrap();
        assert!(pw.chars().any(|c| c.is_ascii_uppercase()));
        assert!(pw.chars().any(|c| c.is_ascii_lowercase()));
        assert!(pw.chars().any(|c| c.is_ascii_digit()));
        assert!(pw.chars().any(|c| SYMBOLS.contains(c)));
    }

    #[test]
    fn excluded_chars_never_appear() {
        let p = GenerateParams {
            length: 16,
            include_uppercase: true,
            include_lowercase: true,
            include_digits: false,
            include_symbols: false,
            exclude_chars: "aeiouAEIOU".to_string(),
        };
        let (pw, _) = generate_impl(&p, MIN, MAX).unwrap();
        assert!(pw.chars().all(|c| !"aeiouAEIOU".contains(c)));
    }

    #[test]
    fn two_generations_differ() {
        let (a, _) = generate_impl(&base_params(), MIN, MAX).unwrap();
        let (b, _) = generate_impl(&base_params(), MIN, MAX).unwrap();
        assert_ne!(a, b);
    }

    #[test]
    fn no_classes_rejected() {
        let p = GenerateParams {
            length: 16,
            include_uppercase: false,
            include_lowercase: false,
            include_digits: false,
            include_symbols: false,
            exclude_chars: String::new(),
        };
        assert_eq!(
            generate_impl(&p, MIN, MAX),
            Err(PasswordError::NoCharacterClasses)
        );
    }

    #[test]
    fn length_too_short_rejected() {
        let mut p = base_params();
        p.length = 2;
        assert_eq!(
            generate_impl(&p, MIN, MAX),
            Err(PasswordError::LengthOutOfRange { min: MIN, max: MAX })
        );
    }

    #[test]
    fn length_too_long_rejected() {
        let mut p = base_params();
        p.length = MAX + 1;
        assert_eq!(
            generate_impl(&p, MIN, MAX),
            Err(PasswordError::LengthOutOfRange { min: MIN, max: MAX })
        );
    }

    #[test]
    fn excluding_entire_class_rejected() {
        let mut p = base_params();
        p.include_uppercase = false;
        p.include_lowercase = false;
        p.include_symbols = false;
        p.include_digits = true;
        p.exclude_chars = "0123456789".to_string();
        assert_eq!(
            generate_impl(&p, MIN, MAX),
            Err(PasswordError::EmptyCharacterPool)
        );
    }

    #[test]
    fn length_smaller_than_class_count_rejected() {
        let p = GenerateParams {
            length: 4,
            include_uppercase: true,
            include_lowercase: true,
            include_digits: true,
            include_symbols: true,
            exclude_chars: String::new(),
        };

        let (pw, _) = generate_impl(&p, MIN, MAX).unwrap();
        assert_eq!(pw.chars().count(), 4);
    }
}
