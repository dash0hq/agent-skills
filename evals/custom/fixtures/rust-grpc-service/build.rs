//! Generates the Inventory gRPC client and server from proto/inventory.proto
//! with a vendored protoc, so the build needs no system protobuf compiler.

fn main() -> Result<(), Box<dyn std::error::Error>> {
    let protoc = protoc_bin_vendored::protoc_bin_path()?;
    // SAFETY: build scripts are single-threaded; nothing else reads the
    // environment concurrently.
    unsafe { std::env::set_var("PROTOC", protoc) };
    tonic_prost_build::compile_protos("proto/inventory.proto")?;
    Ok(())
}
