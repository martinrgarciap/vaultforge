use std::env;
use std::net::SocketAddr;

/// Runtime configuration, from environment variables with safe defaults.
///
/// SECURITY: holds no secrets — only a bind address and length limits.
#[derive(Debug, Clone)]
pub struct Config {
    pub bind_addr: SocketAddr,
    pub min_length: u32,
    pub max_length: u32,
}

impl Config {
    /// Load from env, falling back to safe defaults.
    ///
    /// Env vars:
    /// - PASSWORD_SERVICE_BIND_ADDR    (default 127.0.0.1:50053)
    /// - PASSWORD_SERVICE_MIN_LENGTH   (default 4)
    /// - PASSWORD_SERVICE_MAX_LENGTH   (default 256)
    pub fn from_env() -> Result<Self, String> {
        let bind_addr = env::var("PASSWORD_SERVICE_BIND_ADDR")
            .unwrap_or_else(|_| "127.0.0.1:50053".to_string())
            .parse()
            .map_err(|_| "invalid PASSWORD_SERVICE_BIND_ADDR".to_string())?;

        let min_length = parse_u32_env("PASSWORD_SERVICE_MIN_LENGTH", 4)?;
        let max_length = parse_u32_env("PASSWORD_SERVICE_MAX_LENGTH", 256)?;

        if min_length == 0 {
            return Err("PASSWORD_SERVICE_MIN_LENGTH must be at least 1".to_string());
        }
        if max_length < min_length {
            return Err("PASSWORD_SERVICE_MAX_LENGTH must be >= MIN_LENGTH".to_string());
        }

        Ok(Config {
            bind_addr,
            min_length,
            max_length,
        })
    }
}

fn parse_u32_env(key: &str, default: u32) -> Result<u32, String> {
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
        let cfg = Config::from_env().unwrap();
        assert_eq!(cfg.min_length, 4);
        assert_eq!(cfg.max_length, 256);
        assert_eq!(cfg.bind_addr.port(), 50053);
    }
}
