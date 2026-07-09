pub mod pb {
    tonic::include_proto!("vaultforge.password.v1");
}

fn main() {
    println!("password-service scaffold — proto: vaultforge.password.v1");
    // Reference generated types to confirm codegen worked.
    let _g = pb::GenerateRequest {
        length: 16,
        include_uppercase: true,
        include_lowercase: true,
        include_digits: true,
        include_symbols: false,
        exclude_chars: String::new(),
    };
    let _s = pb::CheckStrengthRequest {
        password: String::new(),
    };
    println!("contract types available ✓");
}