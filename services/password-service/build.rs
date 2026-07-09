use std::path::PathBuf;

fn main() -> Result<(), Box<dyn std::error::Error>> {
    // Where to write the descriptor set (in OUT_DIR, alongside generated code).
    let descriptor_path = PathBuf::from(std::env::var("OUT_DIR")?).join("password_descriptor.bin");

    tonic_prost_build::configure()
        .file_descriptor_set_path(&descriptor_path) // emit the descriptor for reflection
        .compile_protos(
            &["../../packages/proto/vaultforge/password/v1/password.proto"],
            &["../../packages/proto"], // include path (proto root)
        )?;

    Ok(())
}
