# Express API Example

Complete example of an Express REST API with OpenTelemetry instrumentation.

## Project Structure

```
express-api/
├── src/
│   ├── instrumentation.ts   # OTel SDK setup
│   ├── app.ts               # Express app
│   ├── routes/
│   │   └── orders.ts        # Order routes
│   ├── services/
│   │   └── order.service.ts # Business logic
│   └── lib/
│       ├── tracer.ts        # Tracer helper
│       ├── metrics.ts       # Metrics setup
│       └── logger.ts        # Logger with trace context
├── package.json
└── tsconfig.json
```

## Installation

```bash
npm install express
npm install @opentelemetry/sdk-node \
  @opentelemetry/auto-instrumentations-node \
  @opentelemetry/exporter-trace-otlp-proto \
  @opentelemetry/exporter-metrics-otlp-proto \
  @opentelemetry/resources \
  @opentelemetry/semantic-conventions \
  pino

npm install -D typescript @types/express tsx
```

## Files

### instrumentation.ts

```typescript
// src/instrumentation.ts
import { NodeSDK } from '@opentelemetry/sdk-node';
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-proto';
import { OTLPMetricExporter } from '@opentelemetry/exporter-metrics-otlp-proto';
import { PeriodicExportingMetricReader } from '@opentelemetry/sdk-metrics';
import { Resource } from '@opentelemetry/resources';
import {
  ATTR_SERVICE_NAME,
  ATTR_SERVICE_VERSION,
  ATTR_DEPLOYMENT_ENVIRONMENT
} from '@opentelemetry/semantic-conventions';

const resource = new Resource({
  [ATTR_SERVICE_NAME]: process.env.OTEL_SERVICE_NAME || 'express-api',
  [ATTR_SERVICE_VERSION]: process.env.npm_package_version || '1.0.0',
  [ATTR_DEPLOYMENT_ENVIRONMENT]: process.env.NODE_ENV || 'development'
});

const sdk = new NodeSDK({
  resource,
  traceExporter: new OTLPTraceExporter({
    url: process.env.OTEL_EXPORTER_OTLP_ENDPOINT
  }),
  metricReader: new PeriodicExportingMetricReader({
    exporter: new OTLPMetricExporter({
      url: process.env.OTEL_EXPORTER_OTLP_ENDPOINT
    }),
    exportIntervalMillis: process.env.NODE_ENV === 'production' ? 60000 : 10000
  }),
  instrumentations: [
    getNodeAutoInstrumentations({
      '@opentelemetry/instrumentation-fs': { enabled: false },
      '@opentelemetry/instrumentation-http': {
        ignoreIncomingPaths: ['/health', '/ready', '/metrics']
      }
    })
  ]
});

sdk.start();
console.log('OpenTelemetry SDK initialized');

const shutdown = async () => {
  await sdk.shutdown();
  process.exit(0);
};

process.on('SIGTERM', shutdown);
process.on('SIGINT', shutdown);
```

### lib/tracer.ts

```typescript
// src/lib/tracer.ts
import { trace } from '@opentelemetry/api';

export const tracer = trace.getTracer('express-api', '1.0.0');
```

### lib/metrics.ts

```typescript
// src/lib/metrics.ts
import { metrics } from '@opentelemetry/api';

const meter = metrics.getMeter('express-api', '1.0.0');

// HTTP metrics (auto-instrumentation provides these, but we can add custom ones)
export const httpRequestsTotal = meter.createCounter('http.server.requests.total', {
  description: 'Total HTTP requests',
  unit: '1'
});

export const httpRequestDuration = meter.createHistogram('http.server.request.duration', {
  description: 'HTTP request duration',
  unit: 'ms'
});

// Business metrics
export const ordersCreated = meter.createCounter('orders.created.total', {
  description: 'Total orders created',
  unit: '1'
});

export const orderValue = meter.createHistogram('orders.value', {
  description: 'Order value distribution',
  unit: 'USD'
});

export const ordersInProgress = meter.createUpDownCounter('orders.in_progress', {
  description: 'Orders currently being processed',
  unit: '1'
});
```

### lib/logger.ts

```typescript
// src/lib/logger.ts
import pino from 'pino';
import { trace, context } from '@opentelemetry/api';

export const logger = pino({
  level: process.env.LOG_LEVEL || 'info',
  mixin() {
    const span = trace.getSpan(context.active());
    if (!span) return {};

    const spanContext = span.spanContext();
    return {
      trace_id: spanContext.traceId,
      span_id: spanContext.spanId
    };
  }
});
```

### services/order.service.ts

```typescript
// src/services/order.service.ts
import { SpanStatusCode } from '@opentelemetry/api';
import { tracer } from '../lib/tracer';
import { logger } from '../lib/logger';
import { ordersCreated, orderValue, ordersInProgress } from '../lib/metrics';

interface CreateOrderInput {
  customerId: string;
  items: Array<{
    productId: string;
    quantity: number;
    price: number;
  }>;
}

interface Order {
  id: string;
  customerId: string;
  items: CreateOrderInput['items'];
  total: number;
  status: 'pending' | 'confirmed' | 'shipped';
  createdAt: Date;
}

// Simulated database
const orders: Map<string, Order> = new Map();

export class OrderService {
  async createOrder(input: CreateOrderInput): Promise<Order> {
    ordersInProgress.add(1);

    return tracer.startActiveSpan('order.create', async (span) => {
      try {
        span.setAttribute('order.customer_id', input.customerId);
        span.setAttribute('order.items_count', input.items.length);

        logger.info({ customerId: input.customerId }, 'Creating new order');

        // Validate
        span.addEvent('validation.start');
        await this.validateOrder(input);
        span.addEvent('validation.complete');

        // Calculate total
        const total = input.items.reduce(
          (sum, item) => sum + item.price * item.quantity,
          0
        );
        span.setAttribute('order.total', total);

        // Create order
        const order: Order = {
          id: `ord_${Date.now()}`,
          customerId: input.customerId,
          items: input.items,
          total,
          status: 'pending',
          createdAt: new Date()
        };

        // Save to "database"
        await this.saveOrder(order);

        span.setAttribute('order.id', order.id);

        // Record metrics
        ordersCreated.add(1, { 'order.status': 'created' });
        orderValue.record(total);

        logger.info({ orderId: order.id, total }, 'Order created successfully');

        return order;
      } catch (error) {
        span.recordException(error as Error);
        span.setStatus({
          code: SpanStatusCode.ERROR,
          message: (error as Error).message
        });
        logger.error({ error: (error as Error).message }, 'Failed to create order');
        throw error;
      } finally {
        ordersInProgress.add(-1);
        span.end();
      }
    });
  }

  async getOrder(id: string): Promise<Order | null> {
    return tracer.startActiveSpan('order.get', async (span) => {
      try {
        span.setAttribute('order.id', id);

        const order = orders.get(id);

        if (!order) {
          span.setAttribute('order.found', false);
          return null;
        }

        span.setAttribute('order.found', true);
        span.setAttribute('order.status', order.status);

        return order;
      } finally {
        span.end();
      }
    });
  }

  private async validateOrder(input: CreateOrderInput): Promise<void> {
    return tracer.startActiveSpan('order.validate', async (span) => {
      try {
        if (!input.customerId) {
          throw new Error('Customer ID is required');
        }

        if (!input.items || input.items.length === 0) {
          throw new Error('Order must have at least one item');
        }

        for (const item of input.items) {
          if (item.quantity <= 0) {
            throw new Error('Item quantity must be positive');
          }
          if (item.price < 0) {
            throw new Error('Item price cannot be negative');
          }
        }

        span.setAttribute('validation.passed', true);
      } catch (error) {
        span.setAttribute('validation.passed', false);
        throw error;
      } finally {
        span.end();
      }
    });
  }

  private async saveOrder(order: Order): Promise<void> {
    return tracer.startActiveSpan('order.save', async (span) => {
      try {
        span.setAttribute('order.id', order.id);

        // Simulate database latency
        await new Promise((resolve) => setTimeout(resolve, 10));

        orders.set(order.id, order);

        span.addEvent('order.persisted');
      } finally {
        span.end();
      }
    });
  }
}

export const orderService = new OrderService();
```

### routes/orders.ts

```typescript
// src/routes/orders.ts
import { Router, Request, Response, NextFunction } from 'express';
import { orderService } from '../services/order.service';
import { logger } from '../lib/logger';

const router = Router();

// Create order
router.post('/', async (req: Request, res: Response, next: NextFunction) => {
  try {
    const order = await orderService.createOrder(req.body);
    res.status(201).json(order);
  } catch (error) {
    next(error);
  }
});

// Get order
router.get('/:id', async (req: Request, res: Response, next: NextFunction) => {
  try {
    const order = await orderService.getOrder(req.params.id);

    if (!order) {
      res.status(404).json({ error: 'Order not found' });
      return;
    }

    res.json(order);
  } catch (error) {
    next(error);
  }
});

export default router;
```

### app.ts

```typescript
// src/app.ts
import express, { Request, Response, NextFunction } from 'express';
import orderRoutes from './routes/orders';
import { logger } from './lib/logger';
import { httpRequestDuration } from './lib/metrics';

const app = express();

// Middleware
app.use(express.json());

// Request logging and metrics middleware
app.use((req: Request, res: Response, next: NextFunction) => {
  const startTime = Date.now();

  res.on('finish', () => {
    const duration = Date.now() - startTime;

    // Record custom metrics
    httpRequestDuration.record(duration, {
      'http.method': req.method,
      'http.route': req.route?.path || req.path,
      'http.status_code': res.statusCode
    });

    logger.info({
      method: req.method,
      path: req.path,
      statusCode: res.statusCode,
      duration
    }, 'Request completed');
  });

  next();
});

// Health check
app.get('/health', (req, res) => {
  res.json({ status: 'healthy' });
});

// Routes
app.use('/orders', orderRoutes);

// Error handler
app.use((err: Error, req: Request, res: Response, next: NextFunction) => {
  logger.error({ error: err.message, stack: err.stack }, 'Unhandled error');

  res.status(500).json({
    error: 'Internal server error',
    message: process.env.NODE_ENV === 'development' ? err.message : undefined
  });
});

const port = process.env.PORT || 3000;

app.listen(port, () => {
  logger.info({ port }, 'Server started');
});
```

## Running the Application

### Development

```bash
# Set environment variables
export OTEL_SERVICE_NAME=express-api
export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318

# Run with tsx
npx tsx --import ./src/instrumentation.ts src/app.ts
```

### Production

```bash
# Build
npx tsc

# Run
node --import ./dist/instrumentation.js dist/app.js
```

### Docker

```dockerfile
FROM node:20-alpine

WORKDIR /app
COPY package*.json ./
RUN npm ci --only=production
COPY dist ./dist

ENV NODE_ENV=production
ENV OTEL_SERVICE_NAME=express-api

CMD ["node", "--import", "./dist/instrumentation.js", "dist/app.js"]
```

## Testing

### Create an Order

```bash
curl -X POST http://localhost:3000/orders \
  -H "Content-Type: application/json" \
  -d '{
    "customerId": "cust_123",
    "items": [
      {"productId": "prod_1", "quantity": 2, "price": 29.99},
      {"productId": "prod_2", "quantity": 1, "price": 49.99}
    ]
  }'
```

### Get an Order

```bash
curl http://localhost:3000/orders/ord_1234567890
```

## Expected Traces

```
POST /orders
├── middleware (express)
├── request handler - /orders (express)
│   └── order.create
│       ├── order.validate
│       └── order.save
```

## Expected Metrics

```
http.server.request.duration{method=POST, route=/orders, status_code=201}
orders.created.total{order.status=created}
orders.value (histogram)
orders.in_progress (gauge)
```

## Expected Logs

```json
{"level":"info","time":1234567890,"trace_id":"abc...","span_id":"def...","customerId":"cust_123","msg":"Creating new order"}
{"level":"info","time":1234567891,"trace_id":"abc...","span_id":"ghi...","orderId":"ord_123","total":109.97,"msg":"Order created successfully"}
{"level":"info","time":1234567892,"trace_id":"abc...","span_id":"jkl...","method":"POST","path":"/orders","statusCode":201,"duration":45,"msg":"Request completed"}
```
