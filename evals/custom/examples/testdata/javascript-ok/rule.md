# JavaScript and TypeScript project assembly fixture

Create the telemetry helper:

```typescript
// src/lib/telemetry.ts
import { trace, type Tracer } from '@opentelemetry/api';

export function getTracer(name = 'checkout'): Tracer {
  return trace.getTracer(name);
}
```

Use it through the `@/` path alias; `orderId` is application state the example does not define:

```typescript
import { SpanStatusCode } from '@opentelemetry/api';
import { getTracer } from '@/lib/telemetry';

getTracer().startActiveSpan('order.place', (span) => {
  span.setAttribute('order.id', orderId);
  span.setStatus({ code: SpanStatusCode.OK });
  span.end();
});
```

The same in JavaScript:

```javascript
import { trace } from '@opentelemetry/api';

const span = trace.getActiveSpan();
span?.setAttribute('enduser.id', 'TEST-0001');
span?.setAttribute('server.address', 'shop.example.test');
```

A snippet without imports is a fragment and is not checked:

```typescript
span.setAttribute('order.id', orderId);
```
