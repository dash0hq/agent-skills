// Eval fixture: deliberately uninstrumented static web app (see
// evals/fixtures/README.md for the contract, including the browser fixture
// exception). The server serves the static page from static/, answers
// GET /checkout and GET /checkout-data with a JSON checkout response backed
// by one outbound HTTP call to DOWNSTREAM_URL, and listens on PORT (default
// 8080). Because browsers cannot read process environment variables, the
// server forwards its EVAL_-prefixed runtime configuration to the page
// through GET /env.js (window.__EVAL_ENV__); the telemetry of this fixture
// comes from the page, not from this server. All data is obviously synthetic
// (user@example.test, TEST-0001). Node built-ins only: no dependencies.
'use strict';

const fs = require('node:fs');
const http = require('node:http');
const path = require('node:path');

const port = Number.parseInt(process.env.PORT || '8080', 10);
const downstreamUrl = process.env.DOWNSTREAM_URL || 'http://localhost:9090/inventory';
const staticDir = path.join(__dirname, 'static');

const contentTypes = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'application/javascript; charset=utf-8',
  '.mjs': 'application/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.map': 'application/json; charset=utf-8',
};

// evalEnv collects the EVAL_-prefixed runtime configuration the page needs.
function evalEnv() {
  const picked = {};
  for (const [key, value] of Object.entries(process.env)) {
    if (key.startsWith('EVAL_')) {
      picked[key] = value;
    }
  }
  return picked;
}

async function checkout(res) {
  let inventory;
  try {
    const response = await fetch(downstreamUrl);
    if (!response.ok) {
      throw new Error(`inventory lookup failed with status ${response.status}`);
    }
    inventory = await response.text();
  } catch (err) {
    console.error(JSON.stringify({ message: 'inventory lookup failed', error: String(err) }));
    res.writeHead(502, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ error: 'inventory unavailable' }));
    return;
  }

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
}

function serveStatic(res, urlPath) {
  const relative = urlPath === '/' ? 'index.html' : urlPath.replace(/^\/+/, '');
  const file = path.normalize(path.join(staticDir, relative));
  if (!file.startsWith(staticDir + path.sep)) {
    res.writeHead(403, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ error: 'forbidden' }));
    return;
  }
  fs.readFile(file, (err, body) => {
    if (err) {
      res.writeHead(404, { 'Content-Type': 'application/json' });
      res.end(JSON.stringify({ error: 'not found' }));
      return;
    }
    const type = contentTypes[path.extname(file)] || 'application/octet-stream';
    res.writeHead(200, { 'Content-Type': type });
    res.end(body);
  });
}

const server = http.createServer(async (req, res) => {
  const urlPath = new URL(req.url, `http://localhost:${port}`).pathname;
  if (req.method !== 'GET') {
    res.writeHead(405, { 'Content-Type': 'application/json' });
    res.end(JSON.stringify({ error: 'method not allowed' }));
    return;
  }
  if (urlPath === '/checkout' || urlPath === '/checkout-data') {
    await checkout(res);
    return;
  }
  if (urlPath === '/env.js') {
    res.writeHead(200, { 'Content-Type': contentTypes['.js'] });
    res.end(`window.__EVAL_ENV__ = ${JSON.stringify(evalEnv())};\n`);
    return;
  }
  serveStatic(res, urlPath);
});

server.listen(port, () => {
  console.log(JSON.stringify({ message: 'browser-service listening', port }));
});
