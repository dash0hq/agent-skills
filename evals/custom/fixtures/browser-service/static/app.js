// Eval fixture: deliberately uninstrumented page logic (see
// evals/fixtures/README.md). On load it fetches the synthetic checkout data
// (order TEST-0001 for user@example.test) from the same-origin
// /checkout-data endpoint, which the server backs with its DOWNSTREAM_URL
// call, and renders the result.
'use strict';

async function loadCheckout() {
  const target = document.getElementById('checkout-result');
  try {
    const response = await fetch('/checkout-data');
    if (!response.ok) {
      throw new Error(`checkout data request failed with status ${response.status}`);
    }
    const data = await response.json();
    target.textContent = JSON.stringify(data, null, 2);
  } catch (err) {
    target.textContent = `checkout failed: ${err}`;
  }
}

loadCheckout();
