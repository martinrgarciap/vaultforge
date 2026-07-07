mod config;
mod error;
mod grpc;
mod hashing;

pub mod pb {
    tonic::include_proto!("vaultforge.hash.v1");
}

// The generated file descriptor set, for gRPC reflection.
const FILE_DESCRIPTOR_SET: &[u8] = include_bytes!(concat!(env!("OUT_DIR"), "/hash_descriptor.bin"));

use tonic::transport::Server;

use crate::config::Config;
use crate::grpc::HashSvc;
use crate::pb::hash_service_server::HashServiceServer;

#[tokio::main]
async fn main() -> Result<(), Box<dyn std::error::Error>> {
    let config = Config::from_env().map_err(|e| {
        eprintln!("configuration error: {e}");
        e
    })?;

    let addr = config.bind_addr;

    // Health/readiness.
    let (health_reporter, health_service) = tonic_health::server::health_reporter();
    health_reporter
        .set_serving::<HashServiceServer<HashSvc>>()
        .await;

    // Reflection: lets clients discover services without .proto files.
    let reflection_service = tonic_reflection::server::Builder::configure()
        .register_encoded_file_descriptor_set(FILE_DESCRIPTOR_SET)
        .register_encoded_file_descriptor_set(tonic_health::pb::FILE_DESCRIPTOR_SET)
        .build_v1()?;

    let svc = HashSvc::new(config);

    println!("hash-service listening on {addr}");

    Server::builder()
        .add_service(health_service)
        .add_service(reflection_service)
        .add_service(HashServiceServer::new(svc))
        .serve_with_shutdown(addr, shutdown_signal())
        .await?;

    println!("hash-service shut down cleanly");
    Ok(())
}

async fn shutdown_signal() {
    let _ = tokio::signal::ctrl_c().await;
    println!("shutdown signal received");
}
