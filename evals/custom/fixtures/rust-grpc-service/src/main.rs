//! rust-grpc-service is a deliberately uninstrumented HTTP service used as an
//! eval fixture (see evals/custom/fixtures/README.md for the contract). It
//! serves GET /checkout, which reserves stock through a gRPC call to the
//! Inventory service this process also hosts on INVENTORY_PORT (default
//! 50051), then performs one outbound HTTP call to DOWNSTREAM_URL. It listens
//! on PORT (default 8080) and uses only obviously synthetic data
//! (user@example.test, TEST-0001).

use std::{env, time::Duration};

use axum::{Json, Router, extract::State, http::StatusCode, routing::get};
use serde::Serialize;
use tonic::{Request, Response, Status, transport::{Channel, Server}};

pub mod inventory {
    tonic::include_proto!("inventory.v1");
}

use inventory::{
    ReserveReply, ReserveRequest,
    inventory_client::InventoryClient,
    inventory_server::{Inventory, InventoryServer},
};

/// The Inventory gRPC service: every reservation succeeds.
#[derive(Default)]
struct InventoryService;

#[tonic::async_trait]
impl Inventory for InventoryService {
    async fn reserve(&self, request: Request<ReserveRequest>) -> Result<Response<ReserveReply>, Status> {
        let request = request.into_inner();
        tracing::info!(order.id = %request.order_id, sku = %request.sku, "stock reserved");
        Ok(Response::new(ReserveReply { reserved: true }))
    }
}

#[derive(Clone)]
struct AppState {
    client: reqwest::Client,
    downstream_url: String,
    inventory: InventoryClient<Channel>,
}

#[derive(Serialize)]
struct CheckoutResponse {
    order_id: &'static str,
    customer_email: &'static str,
    status: &'static str,
    inventory: String,
}

async fn checkout(State(state): State<AppState>) -> Result<Json<CheckoutResponse>, (StatusCode, &'static str)> {
    let mut inventory = state.inventory.clone();
    let reply = inventory
        .reserve(ReserveRequest { order_id: "TEST-0001".into(), sku: "TEST-SKU-0001".into(), quantity: 1 })
        .await
        .map_err(|status| {
            tracing::error!(error = %status, "stock reservation failed");
            (StatusCode::BAD_GATEWAY, "inventory unavailable")
        })?
        .into_inner();
    if !reply.reserved {
        return Err((StatusCode::CONFLICT, "out of stock"));
    }

    let response = state.client.get(&state.downstream_url).send().await.map_err(|err| {
        tracing::error!(error = %err, "inventory lookup failed");
        (StatusCode::BAD_GATEWAY, "inventory unavailable")
    })?;
    let inventory = response
        .text()
        .await
        .map_err(|_| (StatusCode::BAD_GATEWAY, "inventory unavailable"))?;

    tracing::info!(order.id = "TEST-0001", customer.email = "user@example.test", "checkout completed");

    Ok(Json(CheckoutResponse {
        order_id: "TEST-0001",
        customer_email: "user@example.test",
        status: "confirmed",
        inventory,
    }))
}

#[tokio::main]
async fn main() {
    // One JSON object per line on stdout, with the message as a top-level field.
    tracing_subscriber::fmt().json().flatten_event(true).init();

    let port = env::var("PORT").unwrap_or_else(|_| "8080".to_string());
    let inventory_port = env::var("INVENTORY_PORT").unwrap_or_else(|_| "50051".to_string());
    let downstream_url =
        env::var("DOWNSTREAM_URL").unwrap_or_else(|_| "http://localhost:9090/inventory".to_string());

    let inventory_addr = format!("127.0.0.1:{inventory_port}").parse().expect("inventory address");
    tokio::spawn(async move {
        Server::builder()
            .add_service(InventoryServer::new(InventoryService))
            .serve(inventory_addr)
            .await
            .expect("inventory gRPC server");
    });

    // Connect lazily: the gRPC server above may not be listening yet.
    let inventory = InventoryClient::new(
        Channel::from_shared(format!("http://127.0.0.1:{inventory_port}"))
            .expect("inventory URL")
            .connect_lazy(),
    );
    let client = reqwest::Client::builder()
        .timeout(Duration::from_secs(5))
        .build()
        .expect("HTTP client");

    let app = Router::new()
        .route("/checkout", get(checkout))
        .with_state(AppState { client, downstream_url, inventory });

    let addr = format!("0.0.0.0:{port}");
    let listener = tokio::net::TcpListener::bind(&addr).await.expect("bind");
    tracing::info!(addr = %addr, "rust-grpc-service listening");
    axum::serve(listener, app).await.expect("server");
}
