//! Checkout handler: looks up inventory with one outbound HTTPS call and
//! returns a synthetic order.

use lambda_runtime::{Error, LambdaEvent, service_fn};
use serde_json::{Value, json};

const INVENTORY_URL: &str = "https://checkip.amazonaws.com/";

async fn handler(client: &reqwest::Client, _event: LambdaEvent<Value>) -> Result<Value, Error> {
    let inventory = client
        .get(INVENTORY_URL)
        .send()
        .await?
        .error_for_status()?
        .text()
        .await?;
    println!(r#"{{"message":"checkout completed","order.id":"TEST-0001"}}"#);
    Ok(json!({
        "order_id": "TEST-0001",
        "customer_email": "user@example.test",
        "status": "confirmed",
        "inventory": inventory.trim(),
    }))
}

#[tokio::main]
async fn main() -> Result<(), Error> {
    let client = reqwest::Client::builder()
        .timeout(std::time::Duration::from_secs(5))
        .build()?;
    let client = &client;
    lambda_runtime::run(service_fn(move |event| async move { handler(client, event).await })).await
}
