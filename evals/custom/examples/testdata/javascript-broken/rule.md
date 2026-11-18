# Broken JavaScript project fixture

```javascript
import { trace } from '@opentelemetry/api';

trace.getActiveSpan()?.setAttribute('enduser.id', 'TEST-0001');
```

```typescript
import { trace, SpanStatusCode } from '@opentelemetry/api';

const span = trace.getTracer('checkout').startSpan('order.place');
span.setStatus({ code: SpanStatusCode.FAILED });
span.end();
```
