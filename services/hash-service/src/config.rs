use std::env;
use std::net::SocketAddr;

/// Runtime configuration, read from environment variables with safe defaults.
///
/// SECURITY: this config holds no secrets — only limits, an address, and a
/// concurrency bound. Nothing here is sensitive or logged as sensitive.
#[derive(Debug, Clone)]
pub struct Config {
    /// Address the gRPC server binds to.
    pub bind_addr: SocketAddr,

    /// Maximum accepted password length in bytes.
    pub max_password_len: usize,

    /// Maximum accepted PHC hash length in bytes.
    pub max_hash_len: usize,

    /// Maximum number of concurrent Argon2 operations (bounded concurrency).
    pub max_concurrent_hashes: usize,
}

impl Config {
    /// Load config from the environment, falling back to safe defaults.
    ///
    /// Env vars:
    /// - HASH_SERVICE_BIND_ADDR         (default 127.0.0.1:50051)
    /// - HASH_SERVICE_MAX_PASSWORD_LEN  (default 1024)
    /// - HASH_SERVICE_MAX_HASH_LEN      (default 512)
    /// - HASH_SERVICE_MAX_CONCURRENT    (default 8)
    pub fn from_env() -> Result<Self, String> {
        let bind_addr = env::var("HASH_SERVICE_BIND_ADDR")
            .unwrap_or_else(|_| "127.0.0.1:50051".to_string())
            .parse()
            .map_err(|_| "invalid HASH_SERVICE_BIND_ADDR".to_string())?;

        let max_password_len = parse_usize_env("HASH_SERVICE_MAX_PASSWORD_LEN", 1024)?;
        let max_hash_len = parse_usize_env("HASH_SERVICE_MAX_HASH_LEN", 512)?;
        let max_concurrent_hashes = parse_usize_env("HASH_SERVICE_MAX_CONCURRENT", 8)?;

        if max_concurrent_hashes == 0 {
            return Err("HASH_SERVICE_MAX_CONCURRENT must be at least 1".to_string());
        }

        Ok(Config {
            bind_addr,
            max_password_len,
            max_hash_len,
            max_concurrent_hashes,
        })
    }
}

/// Parse a usize from an env var, using `default` if unset. Errors on garbage.
fn parse_usize_env(key: &str, default: usize) -> Result<usize, String> {
    match env::var(key) {
        Ok(val) => val.parse().map_err(|_| format!("invalid {key}")),
        Err(_) => Ok(default),
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn defaults_are_sane() {
        // With no env vars set, defaults should load and be reasonable.
        // (Tests run without the env vars set, so this exercises the fallbacks.)
        let cfg = Config::from_env().unwrap();
        assert_eq!(cfg.max_password_len, 1024);
        assert_eq!(cfg.max_hash_len, 512);
        assert!(cfg.max_concurrent_hashes >= 1);
        assert_eq!(cfg.bind_addr.port(), 50051);
    }
}
