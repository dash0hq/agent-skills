// Eval fixture: deliberately uninstrumented route handler (see
// evals/fixtures/README.md for the contract). GET /checkout performs one
// outbound HTTP call to DOWNSTREAM_URL server-side and returns a JSON
// response with obviously synthetic data (user@example.test, TEST-0001).
// The listen port comes from PORT via the `next start` invocation in the
// Dockerfile.
export const dynamic = 'force-dynamic';

export async function GET() {
  const downstreamUrl = process.env.DOWNSTREAM_URL || 'http://localhost:9090/inventory';

  let inventory;
  try {
    const response = await fetch(downstreamUrl, { cache: 'no-store' });
    if (!response.ok) {
      throw new Error(`inventory lookup failed with status ${response.status}`);
    }
    inventory = await response.text();
  } catch (err) {
    console.error(JSON.stringify({ message: 'inventory lookup failed', error: String(err) }));
    return Response.json({ error: 'inventory unavailable' }, { status: 502 });
  }

  console.log(
    JSON.stringify({
      message: 'checkout completed',
      'order.id': 'TEST-0001',
      'customer.email': 'user@example.test',
    }),
  );

  return Response.json({
    order_id: 'TEST-0001',
    customer_email: 'user@example.test',
    status: 'confirmed',
    inventory,
  });
}
