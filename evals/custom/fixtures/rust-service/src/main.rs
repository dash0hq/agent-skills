//! rust-service is a deliberately uninstrumented HTTP service used as an eval
//! fixture (see evals/custom/fixtures/README.md for the contract). It serves
//! GET /checkout, performs one outbound HTTP call to DOWNSTREAM_URL while
//! handling it, listens on PORT (default 8080), and uses only obviously
//! synthetic data (user@example.test, TEST-0001).

use std::{env, time::Duration};

use axum::{Json, Router, extract::State, http::StatusCode, routing::get};
use serde::Serialize;

#[derive(Clone)]
struct AppState {
    client: reqwest::Client,
    downstream_url: String,
}

#[derive(Serialize)]
struct CheckoutResponse {
    order_id: &'static str,
    customer_email: &'static str,
    status: &'static str,
    inventory: String,
}

async fn checkout(State(state): State<AppState>) -> Result<Json<CheckoutResponse>, (StatusCode, &'static str)> {
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
    let downstream_url =
        env::var("DOWNSTREAM_URL").unwrap_or_else(|_| "http://localhost:9090/inventory".to_string());
    let client = reqwest::Client::builder()
        .timeout(Duration::from_secs(5))
        .build()
        .expect("HTTP client");

    let app = Router::new()
        .route("/checkout", get(checkout))
        .with_state(AppState { client, downstream_url });

    let addr = format!("0.0.0.0:{port}");
    let listener = tokio::net::TcpListener::bind(&addr).await.expect("bind");
    tracing::info!(addr = %addr, "rust-service listening");
    axum::serve(listener, app).await.expect("server");
}
