<?php
// Eval fixture: deliberately uninstrumented PHP HTTP service (see
// evals/fixtures/README.md for the contract). It serves GET /checkout via the
// PHP built-in web server, performs one outbound HTTP call to DOWNSTREAM_URL
// while handling it, and uses only obviously synthetic data
// (user@example.test, TEST-0001). The listen port comes from PORT via the
// php -S invocation in the Dockerfile.
declare(strict_types=1);

$method = $_SERVER['REQUEST_METHOD'] ?? 'GET';
$path = parse_url($_SERVER['REQUEST_URI'] ?? '/', PHP_URL_PATH);

header('Content-Type: application/json');

if ($method !== 'GET' || $path !== '/checkout') {
    http_response_code(404);
    echo json_encode(['error' => 'not found']);
    return;
}

$downstreamUrl = getenv('DOWNSTREAM_URL') ?: 'http://localhost:9090/inventory';
$context = stream_context_create(['http' => ['timeout' => 5]]);
$inventory = @file_get_contents($downstreamUrl, false, $context);
if ($inventory === false) {
    error_log('inventory lookup failed');
    http_response_code(502);
    echo json_encode(['error' => 'inventory unavailable']);
    return;
}

error_log('checkout completed order.id=TEST-0001 customer.email=user@example.test');
echo json_encode([
    'order_id' => 'TEST-0001',
    'customer_email' => 'user@example.test',
    'status' => 'confirmed',
    'inventory' => $inventory,
]);
