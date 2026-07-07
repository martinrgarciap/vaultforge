use std::sync::Arc;

use tokio::sync::Semaphore;
use tonic::{Request, Response, Status};

use crate::config::Config;
use crate::hashing::{hash_password, verify_password};

use crate::pb::hash_service_server::HashService;
use crate::pb::{
    HashPasswordRequest, HashPasswordResponse, VerifyPasswordRequest, VerifyPasswordResponse,
};

/// The concrete service. Holds config and a semaphore that bounds how many
/// Argon2 operations run concurrently (memory/DoS safety).
#[derive(Clone)]
pub struct HashSvc {
    config: Arc<Config>,
    limiter: Arc<Semaphore>,
}

impl HashSvc {
    pub fn new(config: Config) -> Self {
        let limiter = Arc::new(Semaphore::new(config.max_concurrent_hashes));
        Self {
            config: Arc::new(config),
            limiter,
        }
    }
}

#[tonic::async_trait]
impl HashService for HashSvc {
    async fn hash_password(
        &self,
        request: Request<HashPasswordRequest>,
    ) -> Result<Response<HashPasswordResponse>, Status> {
        let password = request.into_inner().password;
        let max_len = self.config.max_password_len;

        // Acquire a concurrency permit (waits if all are in use).
        let _permit = self
            .limiter
            .clone()
            .acquire_owned()
            .await
            .map_err(|_| Status::internal("internal error"))?;

        // Argon2 is CPU + memory heavy → offload off the async runtime.
        let phc_hash = tokio::task::spawn_blocking(move || hash_password(&password, max_len))
            .await
            .map_err(|_| Status::internal("internal error"))??;
        // _permit drops here → releases the slot.

        Ok(Response::new(HashPasswordResponse { phc_hash }))
    }

    async fn verify_password(
        &self,
        request: Request<VerifyPasswordRequest>,
    ) -> Result<Response<VerifyPasswordResponse>, Status> {
        let req = request.into_inner();
        let (password, phc_hash) = (req.password, req.phc_hash);
        let max_pw = self.config.max_password_len;
        let max_hash = self.config.max_hash_len;

        let _permit = self
            .limiter
            .clone()
            .acquire_owned()
            .await
            .map_err(|_| Status::internal("internal error"))?;

        let verified = tokio::task::spawn_blocking(move || {
            verify_password(&password, &phc_hash, max_pw, max_hash)
        })
        .await
        .map_err(|_| Status::internal("internal error"))??;

        Ok(Response::new(VerifyPasswordResponse { verified }))
    }
}
