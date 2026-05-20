# Reliable Telemetry Forwarding for a Payments Microservices Platform

## Problem/Feature Description

A fintech company runs a payments platform composed of a dozen microservices. Each service is already instrumented with OpenTelemetry SDKs that export traces, metrics, and logs via OTLP. The platform team has been asked to deploy a centralized OpenTelemetry Collector that aggregates telemetry from all services and forwards it to their Dash0 observability account.

The team has had incidents in the past caused by Collector crashes during deployment peaks — telemetry was lost because buffered data disappeared when the process restarted. They also discovered that a previous config accidentally logged full telemetry payloads to stdout in production, consuming significant disk I/O. The platform team wants a configuration that survives restarts without data loss, does not leak credentials, and is safe to leave running permanently in production.

The Collector will be deployed as a container with a 512 MiB memory limit. It must accept OTLP telemetry from other containers on the same network. The Dash0 OTLP endpoint is `ingress.eu-west-1.aws.dash0.com:4317` and the auth token will be injected as an environment variable named `DASH0_AUTH_TOKEN`.

## Output Specification

Produce a file named `collector-config.yaml` containing a complete, production-ready OpenTelemetry Collector configuration that:

- Accepts OTLP telemetry (gRPC and HTTP) from networked containers
- Processes and forwards traces, metrics, and logs to Dash0
- Handles transient backend unavailability without data loss
- Has appropriate memory safeguards for the 512 MiB container limit
- Exposes a health check endpoint

Do not include any placeholder tokens or credentials in the configuration file itself.

