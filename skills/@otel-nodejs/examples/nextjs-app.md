# Next.js App Router Example

Complete example of a Next.js application with OpenTelemetry instrumentation.

## Important Considerations

### Next.js + OTel Challenges

```yaml
challenges:
  - Server components render on server (need server-side OTel)
  - Client components render on browser (different telemetry)
  - API routes are serverless-ish (short-lived)
  - Edge runtime has limited OTel support
  - Turbopack dev server requires special handling
```

### Recommended Approach

```yaml
approach:
  - Focus on server-side instrumentation
  - Use Node.js runtime (not Edge) for API routes needing traces
  - Initialize OTel in instrumentation.ts (Next.js 13.4+)
  - Use Vercel's @vercel/otel for easier setup
```

## Project Structure

```
nextjs-app/
├── src/
│   ├── instrumentation.ts      # OTel setup (Next.js loads this automatically)
│   ├── app/
│   │   ├── layout.tsx
│   │   ├── page.tsx
│   │   └── api/
│   │       └── orders/
│   │           └── route.ts    # API route
│   └── lib/
│       ├── tracer.ts
│       └── logger.ts
├── next.config.js
└── package.json
```

## Installation

### Option 1: Using @vercel/otel (Recommended for Vercel)

```bash
npm install @vercel/otel
```

### Option 2: Standard OTel Setup

```bash
npm install @opentelemetry/sdk-node \
  @opentelemetry/auto-instrumentations-node \
  @opentelemetry/exporter-trace-otlp-proto \
  @opentelemetry/resources \
  @opentelemetry/semantic-conventions
```

## Files

### next.config.js

```javascript
/** @type {import('next').NextConfig} */
const nextConfig = {
  // Enable instrumentation hook
  experimental: {
    instrumentationHook: true
  }
};

module.exports = nextConfig;
```

### instrumentation.ts (Using @vercel/otel)

```typescript
// src/instrumentation.ts
export async function register() {
  if (process.env.NEXT_RUNTIME === 'nodejs') {
    const { registerOTel } = await import('@vercel/otel');

    registerOTel({
      serviceName: process.env.OTEL_SERVICE_NAME || 'nextjs-app'
    });
  }
}
```

### instrumentation.ts (Standard OTel)

```typescript
// src/instrumentation.ts
export async function register() {
  // Only initialize on Node.js runtime (not Edge)
  if (process.env.NEXT_RUNTIME === 'nodejs') {
    const { NodeSDK } = await import('@opentelemetry/sdk-node');
    const { getNodeAutoInstrumentations } = await import(
      '@opentelemetry/auto-instrumentations-node'
    );
    const { OTLPTraceExporter } = await import(
      '@opentelemetry/exporter-trace-otlp-proto'
    );
    const { Resource } = await import('@opentelemetry/resources');
    const {
      ATTR_SERVICE_NAME,
      ATTR_SERVICE_VERSION,
      ATTR_DEPLOYMENT_ENVIRONMENT
    } = await import('@opentelemetry/semantic-conventions');

    const sdk = new NodeSDK({
      resource: new Resource({
        [ATTR_SERVICE_NAME]: process.env.OTEL_SERVICE_NAME || 'nextjs-app',
        [ATTR_SERVICE_VERSION]: process.env.npm_package_version || '1.0.0',
        [ATTR_DEPLOYMENT_ENVIRONMENT]: process.env.NODE_ENV || 'development'
      }),
      traceExporter: new OTLPTraceExporter({
        url: process.env.OTEL_EXPORTER_OTLP_ENDPOINT
      }),
      instrumentations: [
        getNodeAutoInstrumentations({
          '@opentelemetry/instrumentation-fs': { enabled: false }
        })
      ]
    });

    sdk.start();
  }
}
```

### lib/tracer.ts

```typescript
// src/lib/tracer.ts
import { trace } from '@opentelemetry/api';

export function getTracer() {
  return trace.getTracer('nextjs-app', '1.0.0');
}
```

### lib/logger.ts

```typescript
// src/lib/logger.ts
import { trace, context } from '@opentelemetry/api';

interface LogContext {
  [key: string]: unknown;
}

function getTraceContext() {
  const span = trace.getSpan(context.active());
  if (!span) return {};

  const spanContext = span.spanContext();
  return {
    trace_id: spanContext.traceId,
    span_id: spanContext.spanId
  };
}

export const logger = {
  info(message: string, ctx: LogContext = {}) {
    console.log(JSON.stringify({
      level: 'info',
      message,
      ...getTraceContext(),
      ...ctx,
      timestamp: new Date().toISOString()
    }));
  },

  error(message: string, ctx: LogContext = {}) {
    console.error(JSON.stringify({
      level: 'error',
      message,
      ...getTraceContext(),
      ...ctx,
      timestamp: new Date().toISOString()
    }));
  },

  warn(message: string, ctx: LogContext = {}) {
    console.warn(JSON.stringify({
      level: 'warn',
      message,
      ...getTraceContext(),
      ...ctx,
      timestamp: new Date().toISOString()
    }));
  }
};
```

### API Route with Tracing

```typescript
// src/app/api/orders/route.ts
import { NextRequest, NextResponse } from 'next/server';
import { SpanStatusCode } from '@opentelemetry/api';
import { getTracer } from '@/lib/tracer';
import { logger } from '@/lib/logger';

// Force Node.js runtime (required for full OTel support)
export const runtime = 'nodejs';

interface CreateOrderBody {
  customerId: string;
  items: Array<{
    productId: string;
    quantity: number;
    price: number;
  }>;
}

export async function POST(request: NextRequest) {
  const tracer = getTracer();

  return tracer.startActiveSpan('api.orders.create', async (span) => {
    try {
      const body: CreateOrderBody = await request.json();

      span.setAttribute('order.customer_id', body.customerId);
      span.setAttribute('order.items_count', body.items.length);

      logger.info('Creating order', { customerId: body.customerId });

      // Validate
      if (!body.customerId || !body.items?.length) {
        span.setStatus({
          code: SpanStatusCode.ERROR,
          message: 'Invalid order data'
        });
        return NextResponse.json(
          { error: 'Invalid order data' },
          { status: 400 }
        );
      }

      // Calculate total
      const total = body.items.reduce(
        (sum, item) => sum + item.price * item.quantity,
        0
      );

      // Create order (simulated)
      const order = await createOrder(body, total);

      span.setAttribute('order.id', order.id);
      span.setAttribute('order.total', order.total);

      logger.info('Order created', { orderId: order.id, total: order.total });

      return NextResponse.json(order, { status: 201 });
    } catch (error) {
      span.recordException(error as Error);
      span.setStatus({
        code: SpanStatusCode.ERROR,
        message: (error as Error).message
      });

      logger.error('Failed to create order', { error: (error as Error).message });

      return NextResponse.json(
        { error: 'Internal server error' },
        { status: 500 }
      );
    } finally {
      span.end();
    }
  });
}

async function createOrder(body: CreateOrderBody, total: number) {
  const tracer = getTracer();

  return tracer.startActiveSpan('order.persist', async (span) => {
    try {
      // Simulate database operation
      await new Promise((resolve) => setTimeout(resolve, 10));

      const order = {
        id: `ord_${Date.now()}`,
        customerId: body.customerId,
        items: body.items,
        total,
        status: 'pending',
        createdAt: new Date().toISOString()
      };

      span.setAttribute('db.operation', 'INSERT');
      span.addEvent('order.saved');

      return order;
    } finally {
      span.end();
    }
  });
}

export async function GET(request: NextRequest) {
  const tracer = getTracer();
  const { searchParams } = new URL(request.url);
  const customerId = searchParams.get('customerId');

  return tracer.startActiveSpan('api.orders.list', async (span) => {
    try {
      if (customerId) {
        span.setAttribute('filter.customer_id', customerId);
      }

      // Simulated response
      const orders = [
        { id: 'ord_1', customerId: 'cust_1', total: 99.99 },
        { id: 'ord_2', customerId: 'cust_1', total: 149.99 }
      ];

      span.setAttribute('orders.count', orders.length);

      return NextResponse.json(orders);
    } finally {
      span.end();
    }
  });
}
```

### Server Component with Tracing

```typescript
// src/app/orders/page.tsx
import { getTracer } from '@/lib/tracer';

// Server Component - runs on server
export default async function OrdersPage() {
  const tracer = getTracer();

  const orders = await tracer.startActiveSpan('page.orders.load', async (span) => {
    try {
      // Fetch orders (server-side)
      const response = await fetch(`${process.env.API_URL}/api/orders`, {
        cache: 'no-store'
      });

      if (!response.ok) {
        throw new Error('Failed to fetch orders');
      }

      const data = await response.json();
      span.setAttribute('orders.count', data.length);

      return data;
    } finally {
      span.end();
    }
  });

  return (
    <div>
      <h1>Orders</h1>
      <ul>
        {orders.map((order: any) => (
          <li key={order.id}>
            {order.id} - ${order.total}
          </li>
        ))}
      </ul>
    </div>
  );
}
```

### Server Action with Tracing

```typescript
// src/app/orders/actions.ts
'use server';

import { SpanStatusCode } from '@opentelemetry/api';
import { getTracer } from '@/lib/tracer';
import { logger } from '@/lib/logger';
import { revalidatePath } from 'next/cache';

export async function createOrderAction(formData: FormData) {
  const tracer = getTracer();

  return tracer.startActiveSpan('action.orders.create', async (span) => {
    try {
      const customerId = formData.get('customerId') as string;
      const productId = formData.get('productId') as string;
      const quantity = parseInt(formData.get('quantity') as string, 10);

      span.setAttribute('order.customer_id', customerId);
      span.setAttribute('order.product_id', productId);

      logger.info('Creating order via server action', { customerId, productId });

      // Call API
      const response = await fetch(`${process.env.API_URL}/api/orders`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          customerId,
          items: [{ productId, quantity, price: 29.99 }]
        })
      });

      if (!response.ok) {
        throw new Error('Failed to create order');
      }

      const order = await response.json();
      span.setAttribute('order.id', order.id);

      // Revalidate orders page
      revalidatePath('/orders');

      return { success: true, orderId: order.id };
    } catch (error) {
      span.recordException(error as Error);
      span.setStatus({
        code: SpanStatusCode.ERROR,
        message: (error as Error).message
      });

      logger.error('Server action failed', { error: (error as Error).message });

      return { success: false, error: (error as Error).message };
    } finally {
      span.end();
    }
  });
}
```

## Environment Variables

```bash
# .env.local
OTEL_SERVICE_NAME=nextjs-app
OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318
API_URL=http://localhost:3000
```

## Running

### Development

```bash
npm run dev
```

### Production

```bash
npm run build
npm start
```

### With Docker

```dockerfile
FROM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN npm ci
COPY . .
RUN npm run build

FROM node:20-alpine AS runner
WORKDIR /app
ENV NODE_ENV=production
COPY --from=builder /app/.next/standalone ./
COPY --from=builder /app/.next/static ./.next/static
COPY --from=builder /app/public ./public

ENV OTEL_SERVICE_NAME=nextjs-app
EXPOSE 3000
CMD ["node", "server.js"]
```

## Expected Traces

### API Route

```
POST /api/orders
└── api.orders.create
    └── order.persist
```

### Server Component

```
GET /orders (page render)
└── page.orders.load
    └── HTTP GET /api/orders (auto-instrumented)
```

### Server Action

```
POST (server action)
└── action.orders.create
    └── HTTP POST /api/orders (auto-instrumented)
```

## Edge Runtime Considerations

```typescript
// For Edge runtime, OTel has limited support
// Use this pattern for basic logging

// src/app/api/edge-route/route.ts
export const runtime = 'edge';

export async function GET(request: Request) {
  // Edge doesn't have full OTel support
  // Use structured logging instead
  console.log(JSON.stringify({
    level: 'info',
    message: 'Edge function invoked',
    timestamp: new Date().toISOString(),
    path: request.url
  }));

  return new Response('OK');
}
```

## Vercel Deployment

When deploying to Vercel, use `@vercel/otel`:

```typescript
// src/instrumentation.ts
export async function register() {
  if (process.env.NEXT_RUNTIME === 'nodejs') {
    const { registerOTel } = await import('@vercel/otel');

    registerOTel({
      serviceName: 'nextjs-app',
      // Vercel automatically configures the exporter
    });
  }
}
```

Configure in Vercel dashboard:
1. Go to Project Settings → Observability
2. Enable OpenTelemetry
3. Configure your OTLP endpoint
