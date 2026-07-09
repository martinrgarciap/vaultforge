// services/password-service/src/main.rs

mod config;
mod error;
mod generator;
mod grpc;
mod strength;

pub mod pb {
    tonic::include_proto!("vaultforge.password.v1");
}

const FILE_DESCRIPTOR_SET: &[u8] =
    include_bytes!(concat!(env!("OUT_DIR"), "/password_descriptor.bin"));

use tonic::transport::Server;

use crate::config::Config;
use crate::grpc::PasswordSvc;
use crate::pb::password_service_server::PasswordServiceServer;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let config = Config::from_env().map_err(|e| {
        eprintln!("configuration error: {e}");
        e
    })?;

    let addr = config.bind_addr;

    let (health_reporter, health_service) = tonic_health::server::health_reporter();
    health_reporter
        .set_serving::<PasswordServiceServer<PasswordSvc>>()
        .await;

    let reflection_service = tonic_reflection::server::Builder::configure()
        .register_encoded_file_descriptor_set(FILE_DESCRIPTOR_SET)
        .register_encoded_file_descriptor_set(tonic_health::pb::FILE_DESCRIPTOR_SET)
        .build_v1()?;

    let svc = PasswordSvc::new(config);

    println!("password-service listening on {addr}");

    Server::builder()
        .add_service(health_service)
        .add_service(reflection_service)
        .add_service(PasswordServiceServer::new(svc))
        .serve_with_shutdown(addr, shutdown_signal())
        .await?;

    println!("password-service shut down cleanly");
    Ok(())
}

async fn shutdown_signal() {
    let _ = tokio::signal::ctrl_c().await;
    println!("shutdown signal received");
}
