'use strict';

// Checkout handler: looks up inventory with one outbound HTTP call and returns
// a synthetic order. There is deliberately no OpenTelemetry code here.

const INVENTORY_URL = 'https://checkip.amazonaws.com/';
const REQUEST_TIMEOUT_MS = 5_000;

exports.handler = async function handler() {
  const response = await fetch(INVENTORY_URL, { signal: AbortSignal.timeout(REQUEST_TIMEOUT_MS) });
  const inventory = (await response.text()).trim();
  console.log(JSON.stringify({ message: 'checkout completed', 'order.id': 'TEST-0001' }));
  return {
    statusCode: 200,
    body: JSON.stringify({
      order_id: 'TEST-0001',
      customer_email: 'user@example.test',
      status: 'confirmed',
      inventory,
    }),
  };
};
