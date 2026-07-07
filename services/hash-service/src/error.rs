use std::fmt;

/// Errors from the hashing module.
///
/// SECURITY: no variant ever carries the password, the hash, or any
/// secret-bearing input — only safe metadata (lengths, limits, static reasons).
/// These map to gRPC statuses in the service layer (see grpc.rs / 14D).
#[derive(Debug, PartialEq)]
pub enum HashError {
    /// Input was empty (password or hash). → invalid_argument
    Empty { field: &'static str },

    /// Input exceeded the configured maximum length. → invalid_argument
    /// Carries only lengths, never the input itself.
    TooLong {
        field: &'static str,
        actual: usize,
        max: usize,
    },

    /// The provided PHC hash string could not be parsed. → invalid_argument
    /// Carries NO detail about the hash contents.
    MalformedHash,

    /// Argon2 failed internally (e.g. bad parameters, allocation). → internal
    /// Carries a static reason, never the password.
    Internal,
}

impl fmt::Display for HashError {
    fn fmt(&self, f: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            HashError::Empty { field } => {
                write!(f, "{field} must not be empty")
            }
            HashError::TooLong { field, actual, max } => {
                write!(f, "{field} too long ({actual} bytes, max {max})")
            }
            HashError::MalformedHash => {
                write!(f, "stored hash is malformed")
            }
            HashError::Internal => {
                write!(f, "internal hashing error")
            }
        }
    }
}

impl std::error::Error for HashError {}

impl From<HashError> for tonic::Status {
    fn from(err: HashError) -> tonic::Status {
        // The Display messages are already secret-free (see above), so it is
        // safe to surface them as the status message.
        match err {
            HashError::Empty { .. } | HashError::TooLong { .. } | HashError::MalformedHash => {
                tonic::Status::invalid_argument(err.to_string())
            }
            HashError::Internal => {
                // Do NOT surface internal detail to clients.
                tonic::Status::internal("internal error")
            }
        }
    }
}
