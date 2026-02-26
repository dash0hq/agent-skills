---
title: "Node.js Instrumentation"
impact: HIGH
tags:
  - nodejs
  - backend
  - server
---

# Node.js Instrumentation

Instrument Node.js applications to generate traces, logs, and metrics for deep insights into behavior and performance.

## Use Cases

- **HTTP Request Monitoring**: Understand outgoing and incoming HTTP requests through traces and metrics, with drill-downs to database level
- **Database Performance**: Observe which database statements execute and measure their duration for optimization
- **Error Detection**: Reveal uncaught errors and the context in which they happened

---

## Installation

```bash
npm install @opentelemetry/auto-instrumentations-node
```

**Note**: Installing the package alone is insufficient—you must activate the SDK.

---

## Configuration

### 1. Activate the SDK

Use `NODE_OPTIONS` to load dependencies at startup:

```bash
export NODE_OPTIONS="--require @opentelemetry/auto-instrumentations-node/register"
```

**Note**: Tools like npm, pnpm, and yarn are Node.js applications, so you may observe instrumentation data from package managers.

### 2. Set Service Name

```bash
export OTEL_SERVICE_NAME="my-service"
```

### 3. Configure Export

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT="https://ingress.eu-west-1.dash0.com:4317"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN"
```

### 4. Optional: Target Specific Dataset

```bash
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN,Dash0-Dataset=my-dataset"
```

---

## Complete Setup

```bash
export OTEL_SERVICE_NAME="my-service"
export OTEL_EXPORTER_OTLP_ENDPOINT="https://ingress.eu-west-1.dash0.com:4317"
export OTEL_EXPORTER_OTLP_HEADERS="Authorization=Bearer YOUR_AUTH_TOKEN"
export NODE_OPTIONS="--require @opentelemetry/auto-instrumentations-node/register"

node app.js
```

---

## Kubernetes Setup

When not using the Dash0 Kubernetes Operator, extend resource attributes with Pod information for proper log, trace, and metrics correlation:

```bash
export OTEL_RESOURCE_ATTRIBUTES="k8s.pod.name=$(hostname),k8s.pod.uid=$(POD_UID)"
```

**Recommended**: Use the [Dash0 Kubernetes Operator](https://github.com/dash0hq/dash0-operator) for automatic instrumentation of Node.js workloads.

---

## Supported Libraries

The auto-instrumentation package automatically instruments:

| Category | Libraries |
|----------|-----------|
| HTTP | http, https, express, fastify, koa, hapi |
| Database | pg, mysql, mysql2, mongodb, redis, ioredis |
| ORM | knex, sequelize, typeorm, prisma |
| Messaging | amqplib, kafkajs |
| AWS | aws-sdk, @aws-sdk/* |
| Logging | pino, winston, bunyan |
| GraphQL | graphql |
| gRPC | @grpc/grpc-js |

Refer to [OpenTelemetry documentation](https://opentelemetry.io/ecosystem/registry/?language=js) for the complete list.

---

## Custom Spans

Add business context to auto-instrumented traces:

```javascript
import { trace } from "@opentelemetry/api";

const tracer = trace.getTracer("my-service");

async function processOrder(order) {
  return tracer.startActiveSpan("order.process", async (span) => {
    try {
      span.setAttribute("order.id", order.id);
      span.setAttribute("order.total", order.total);
      const result = await saveOrder(order);
      return result;
    } catch (error) {
      span.recordException(error);
      throw error;
    } finally {
      span.end();
    }
  });
}
```

---

## Resources

- [OpenTelemetry Node.js Documentation](https://opentelemetry.io/docs/languages/js/getting-started/nodejs/)
- [Auto-Instrumentation Package](https://www.npmjs.com/package/@opentelemetry/auto-instrumentations-node)
- [Dash0 Kubernetes Operator](https://github.com/dash0hq/dash0-operator)
