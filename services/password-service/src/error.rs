use std::fmt;

/// Errors from the password service (generation + strength).
///
/// SECURITY: no variant carries the password or any secret-bearing input —
/// only safe, static descriptions and numeric limits. These map to gRPC
/// statuses in the service layer (grpc.rs).
#[derive(Debug, PartialEq, Eq)]
pub enum PasswordError {
    /// Requested length was outside the allowed range. → invalid_argument
    LengthOutOfRange { min: u32, max: u32 },

    /// No character classes were enabled. → invalid_argument
    NoCharacterClasses,

    /// An enabled class had no usable characters after exclusions. → invalid_argument
    EmptyCharacterPool,

    /// Requested length is smaller than the number of enabled classes,
    /// so we can't guarantee one char from each. → invalid_argument
    LengthTooSmallForClasses { length: u32, classes: u32 },

    /// An internal error occurred during generation. → internal
    Internal,
}

impl fmt::Display for PasswordError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            PasswordError::LengthOutOfRange { min, max } => {
                write!(f, "length must be between {min} and {max}")
            }
            PasswordError::NoCharacterClasses => {
                write!(f, "at least one character class must be enabled")
            }
            PasswordError::EmptyCharacterPool => {
                write!(f, "no usable characters remain after exclusions")
            }
            PasswordError::LengthTooSmallForClasses { length, classes } => {
                write!(
                    f,
                    "length {length} is too small to include all {classes} enabled character classes"
                )
            }
            PasswordError::Internal => write!(f, "internal error"),
        }
    }
}

impl std::error::Error for PasswordError {}

impl From<PasswordError> for tonic::Status {
    fn from(err: PasswordError) -> tonic::Status {
        match err {
            PasswordError::LengthOutOfRange { .. }
            | PasswordError::NoCharacterClasses
            | PasswordError::EmptyCharacterPool
            | PasswordError::LengthTooSmallForClasses { .. } => {
                tonic::Status::invalid_argument(err.to_string())
            }
            PasswordError::Internal => tonic::Status::internal("internal error"),
        }
    }
}
