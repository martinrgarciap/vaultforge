// services/password-service/build.rs
fn main() -> Result<(), Box<dyn std::error::Error>> {
    tonic_prost_build::compile_protos(
        "../../packages/proto/vaultforge/password/v1/password.proto",
    )?;
    Ok(())
}