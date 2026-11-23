# Eval fixture: uninstrumented Rust HTTP service (see evals/fixtures/README.md).
# Deterministic build: pinned base images and a committed Cargo.lock.
#
#   docker build -t rust-service-eval .
FROM rust:1.99-slim-trixie AS build
WORKDIR /src
COPY . .
RUN cargo build --release

FROM debian:trixie-slim
COPY --from=build /src/target/release/rust-service /usr/local/bin/rust-service
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/rust-service"]
