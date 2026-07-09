use crate::error::PasswordError;

pub struct StrengthResult {
    pub score: u32,    // 0-4
    pub label: String, // human-readable
    pub entropy_bits: f64,
    pub crack_time_estimate: String, // human-readable
    pub suggestions: Vec<String>,
}

/// Rate a password's strength using zxcvbn.
///
/// SECURITY: the password is passed to zxcvbn for analysis only; it is never
/// logged, stored, or echoed. Only the derived rating is returned.
pub fn check_strength_impl(password: &str) -> Result<StrengthResult, PasswordError> {
    let estimate = zxcvbn::zxcvbn(password, &[]);

    // score() returns a 0-4 Score enum; convert to u32.
    let score_u8 = u8::from(estimate.score());
    let score = score_u8 as u32;

    let label = label_for_score(score);

    let guesses = estimate.guesses();
    let entropy_bits = (guesses.max(1) as f64).log2();

    // Human-readable crack time (offline fast-hash scenario is a reasonable default).
    let crack_time_estimate = estimate
        .crack_times()
        .offline_fast_hashing_1e10_per_second()
        .to_string();

    // Suggestions from feedback, if any.
    let suggestions = estimate
        .feedback()
        .map(|fb| {
            fb.suggestions()
                .iter()
                .map(|s| s.to_string())
                .collect::<Vec<String>>()
        })
        .unwrap_or_default();

    Ok(StrengthResult {
        score,
        label,
        entropy_bits,
        crack_time_estimate,
        suggestions,
    })
}

/// Map a 0-4 score to a human-readable label.
fn label_for_score(score: u32) -> String {
    match score {
        0 => "very weak",
        1 => "weak",
        2 => "fair",
        3 => "strong",
        4 => "very strong",
        _ => "unknown",
    }
    .to_string()
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn weak_password_scores_low() {
        let r = check_strength_impl("password123").unwrap();
        assert!(r.score <= 1, "expected low score, got {}", r.score);
    }

    #[test]
    fn strong_password_scores_high() {
        let r = check_strength_impl("correct-horse-battery-staple-9f3k2").unwrap();
        assert!(r.score >= 3, "expected high score, got {}", r.score);
    }

    #[test]
    fn score_has_matching_label() {
        let r = check_strength_impl("aaa").unwrap();
        assert!(!r.label.is_empty());
        assert!(r.score <= 1);
    }

    #[test]
    fn returns_entropy_and_crack_time() {
        let r = check_strength_impl("Tr0ub4dour&3").unwrap();
        assert!(r.entropy_bits > 0.0);
        assert!(!r.crack_time_estimate.is_empty());
    }

    #[test]
    fn label_mapping_is_correct() {
        assert_eq!(label_for_score(0), "very weak");
        assert_eq!(label_for_score(4), "very strong");
    }
}
