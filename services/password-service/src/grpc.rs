use std::sync::Arc;

use tonic::{Request, Response, Status};

use crate::config::Config;
use crate::generator::{generate_impl, GenerateParams};
use crate::strength::check_strength_impl;

use crate::pb::password_service_server::PasswordService;
use crate::pb::{CheckStrengthRequest, CheckStrengthResponse, GenerateRequest, GenerateResponse};

#[derive(Debug)]
pub struct PasswordSvc {
    config: Arc<Config>,
}

impl PasswordSvc {
    pub fn new(config: Config) -> Self {
        Self {
            config: Arc::new(config),
        }
    }
}

#[tonic::async_trait]
impl PasswordService for PasswordSvc {
    async fn generate(
        &self,
        request: Request<GenerateRequest>,
    ) -> Result<Response<GenerateResponse>, Status> {
        let req = request.into_inner();

        let params = GenerateParams {
            length: req.length,
            include_uppercase: req.include_uppercase,
            include_lowercase: req.include_lowercase,
            include_digits: req.include_digits,
            include_symbols: req.include_symbols,
            exclude_chars: req.exclude_chars,
        };

        let (password, entropy_bits) =
            generate_impl(&params, self.config.min_length, self.config.max_length)?;

        Ok(Response::new(GenerateResponse {
            password,
            entropy_bits,
        }))
    }

    async fn check_strength(
        &self,
        request: Request<CheckStrengthRequest>,
    ) -> Result<Response<CheckStrengthResponse>, Status> {
        let password = request.into_inner().password;

        let result = check_strength_impl(&password)?;

        Ok(Response::new(CheckStrengthResponse {
            score: result.score,
            label: result.label,
            entropy_bits: result.entropy_bits,
            crack_time_estimate: result.crack_time_estimate,
            suggestions: result.suggestions,
        }))
    }
}
