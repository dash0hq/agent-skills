# Checkout Frontend Browser Monitoring

## Problem Description

Harbourside Outfitters runs a single-page checkout at `https://checkout.example.test`, built with Vite and plain TypeScript. The checkout collects a shipping address, an email address, and card details, and it posts them to an internal payments API.

The frontend team has no visibility into what happens in the browser. Backend services are already instrumented and export to Dash0, but when a customer reports that "the checkout was broken," nobody can tell whether the page failed to load, a request to the payments API hung, or a script threw.

The team also wants to understand why customers abandon the final step of the checkout. Support tickets describe customers who "clicked the pay button over and over and nothing happened," and nobody can tell what those customers were actually looking at when it happened.

The team has an ingest endpoint and an auth token provisioned for this work:

- OTLP via HTTP endpoint: `https://ingress.example.test`
- Auth token: `TEST-INGEST-0001`
- Required `service.name`: `checkout-web`

## Output Specification

Produce the following files:

1. **`src/instrumentation.ts`** — The browser telemetry setup for the checkout, exporting to the endpoint above under the required service name.

2. **`index.html`** — The checkout page shell, showing where the instrumentation is loaded relative to the rest of the page's scripts.

3. **`NOTES.md`** — A short handover note for the frontend team covering what the instrumentation now captures, what it does not capture, and anything the team must decide or act on before this ships.

Use the published Dash0 and OpenTelemetry browser packages as they exist on npm. Keep the endpoint and token values above as literal placeholders in the code.
