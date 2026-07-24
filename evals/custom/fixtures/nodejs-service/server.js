// Eval fixture: deliberately uninstrumented Node.js HTTP service (see
// evals/fixtures/README.md for the contract). It serves GET /checkout,
// performs one outbound HTTP call to DOWNSTREAM_URL while handling it,
// listens on PORT (default 8080), and uses only obviously synthetic data
// (user@example.test, TEST-0001). Node built-ins only: no dependencies.
'use strict';

const http = require('node:http');

const port = Number.parseInt(process.env.PORT || '8080', 10);
const downstreamUrl = process.env.DOWNSTREAM_URL || 'http://localhost:9090/inventory';

async function fetchInventory() {
  const response = await fetch(downstreamUrl);
  if (!response.ok) {
    throw new Error(`inventory lookup failed with status ${response.status}`);
  }
  return response.text();
}

const server = http.createServer(async (req, res) => {
  if (req.method !== 'GET' || req.url !== '/checkout') {
    res.writeHead(404, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ error: 'not found' }));
    return;
  }

  try {
    const inventory = await fetchInventory();
    console.log(
      JSON.stringify({
        message: 'checkout completed',
        'order.id': 'TEST-0001',
        'customer.email': 'user@example.test',
      }),
    );
    res.writeHead(200, { 'Content-Type': 'application/json' });
    res.end(
      JSON.stringify({
        order_id: 'TEST-0001',
        customer_email: 'user@example.test',
        status: 'confirmed',
        inventory,
      }),
    );
  } catch (err) {
    console.error(JSON.stringify({ message: 'inventory lookup failed', error: String(err) }));
    res.writeHead(502, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ error: 'inventory unavailable' }));
  }
});

server.listen(port, () => {
  console.log(JSON.stringify({ message: 'nodejs-service listening', port }));
});
