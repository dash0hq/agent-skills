# SLO-Accurate RED Metrics Alongside Trace Volume Reduction

## Problem/Feature Description

A B2B SaaS company's platform engineering team manages a high-traffic API gateway that processes around 2,000 requests per second. Their observability setup exports every trace to their backend, which is generating significant storage costs. The team wants to implement sampling to reduce trace volume by roughly 90%, while retaining all error traces, traces for requests slower than 1 second, and a 10% random baseline of healthy traffic.

The team also runs service-level dashboards and alerts based on request rate, error rate, and duration (RED metrics). They previously tried head sampling but discovered that their dashboards were showing incorrect error rates because sampled traces underrepresented errors. A colleague suggested computing RED metrics from spans before any sampling decision is made.

The platform team needs a gateway Collector configuration that:
- Implements intelligent, content-aware trace sampling (keeping errors, slow requests, and a baseline of normal traffic)
- Derives accurate RED metrics from HTTP server spans, using metric names that match standard OpenTelemetry semantic conventions so they can be correlated with SDK-emitted metrics
- Exports both the (sampled) traces and the derived metrics to Dash0

The Dash0 OTLP endpoint is `ingress.eu-west-1.aws.dash0.com:4317` and the auth token will be injected as an environment variable named `DASH0_AUTH_TOKEN`.

## Output Specification

Produce a file named `collector-config.yaml` containing the gateway Collector configuration.

Also produce a file named `notes.md` that explains:
1. Why RED metrics must be computed before sampling (not after)
2. What sampler setting you recommend for the application SDKs, and why

