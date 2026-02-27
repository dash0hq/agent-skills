import { NextRequest, NextResponse } from "next/server";
import { getTracer, getMeter, logger, withSpan, getTraceContext } from "@/lib/telemetry";
import { SpanStatusCode } from "@opentelemetry/api";

// Create metrics instruments (do this once at module level)
const meter = getMeter();
const requestCounter = meter.createCounter("demo.requests", {
  description: "Total number of demo API requests",
  unit: "1",
});
const requestDuration = meter.createHistogram("demo.request.duration", {
  description: "Duration of demo API requests",
  unit: "ms",
});
const activeRequests = meter.createUpDownCounter("demo.requests.active", {
  description: "Number of active demo API requests",
  unit: "1",
});

// Simulated data processing function
async function processData(data: { name: string; value: number }) {
  return withSpan(
    "demo.process_data",
    {
      "data.name": data.name,
      "data.value": data.value,
    },
    async () => {
      // Simulate some processing time
      await new Promise((resolve) => setTimeout(resolve, 50 + Math.random() * 100));

      logger.info("data.processed", {
        data_name: data.name,
        data_value: data.value,
        processing_result: "success",
      });

      return {
        processed: true,
        result: data.value * 2,
        timestamp: new Date().toISOString(),
      };
    }
  );
}

// Simulated database operation
async function fetchFromDatabase(id: string) {
  return withSpan(
    "demo.db.fetch",
    {
      "db.operation": "SELECT",
      "db.table": "items",
      "item.id": id,
    },
    async () => {
      // Simulate DB latency
      await new Promise((resolve) => setTimeout(resolve, 20 + Math.random() * 30));

      logger.info("db.query.executed", {
        operation: "SELECT",
        table: "items",
        item_id: id,
        rows_returned: 1,
      });

      return {
        id,
        name: `Item ${id}`,
        value: Math.floor(Math.random() * 100),
        createdAt: new Date().toISOString(),
      };
    }
  );
}

export async function GET(request: NextRequest) {
  const tracer = getTracer();
  const startTime = Date.now();

  // Track active requests
  activeRequests.add(1);

  return tracer.startActiveSpan("demo.api.get", async (span) => {
    try {
      // Extract query parameters
      const searchParams = request.nextUrl.searchParams;
      const itemId = searchParams.get("id") || "default-123";
      const includeProcessing = searchParams.get("process") === "true";

      // Set span attributes
      span.setAttribute("http.route", "/api/demo");
      span.setAttribute("item.id", itemId);
      span.setAttribute("include_processing", includeProcessing);

      // Log the incoming request
      logger.info("api.request.received", {
        route: "/api/demo",
        method: "GET",
        item_id: itemId,
        include_processing: includeProcessing,
      });

      // Increment request counter with attributes (bounded cardinality)
      requestCounter.add(1, {
        method: "GET",
        route: "/api/demo",
        status: "started",
      });

      // Fetch data from "database"
      const item = await fetchFromDatabase(itemId);

      let result = item;
      if (includeProcessing) {
        const processed = await processData(item);
        result = { ...item, ...processed };
      }

      // Get trace context for response headers (enables client correlation)
      const traceContext = getTraceContext();

      // Log successful completion
      logger.info("api.request.completed", {
        route: "/api/demo",
        method: "GET",
        item_id: itemId,
        duration_ms: Date.now() - startTime,
        status: "success",
      });

      // Record duration metric
      requestDuration.record(Date.now() - startTime, {
        method: "GET",
        route: "/api/demo",
        status_code: "200",
      });

      // Update request counter for success
      requestCounter.add(1, {
        method: "GET",
        route: "/api/demo",
        status: "success",
      });

      return NextResponse.json(
        {
          success: true,
          data: result,
          // Include trace context in response for debugging
          _trace: {
            traceId: traceContext.traceId,
            spanId: traceContext.spanId,
          },
        },
        {
          headers: {
            // Expose trace ID for client-side correlation
            "X-Trace-Id": traceContext.traceId || "",
          },
        }
      );
    } catch (error) {
      // Record exception in span
      span.recordException(error as Error);
      span.setStatus({ code: SpanStatusCode.ERROR });

      // Log error with context
      logger.error("api.request.failed", {
        route: "/api/demo",
        method: "GET",
        error_type: (error as Error).name,
        error_message: (error as Error).message,
        duration_ms: Date.now() - startTime,
      });

      // Record failure metrics
      requestDuration.record(Date.now() - startTime, {
        method: "GET",
        route: "/api/demo",
        status_code: "500",
      });
      requestCounter.add(1, {
        method: "GET",
        route: "/api/demo",
        status: "error",
      });

      return NextResponse.json(
        { success: false, error: "Internal server error" },
        { status: 500 }
      );
    } finally {
      activeRequests.add(-1);
      span.end();
    }
  });
}

export async function POST(request: NextRequest) {
  const tracer = getTracer();
  const startTime = Date.now();

  activeRequests.add(1);

  return tracer.startActiveSpan("demo.api.post", async (span) => {
    try {
      const body = await request.json();
      const { name, value } = body;

      span.setAttribute("http.route", "/api/demo");
      span.setAttribute("item.name", name || "unknown");

      logger.info("api.request.received", {
        route: "/api/demo",
        method: "POST",
        item_name: name,
        item_value: value,
      });

      requestCounter.add(1, {
        method: "POST",
        route: "/api/demo",
        status: "started",
      });

      // Validate input
      if (!name || typeof value !== "number") {
        logger.warn("api.validation.failed", {
          route: "/api/demo",
          method: "POST",
          reason: "Missing name or invalid value",
        });

        requestCounter.add(1, {
          method: "POST",
          route: "/api/demo",
          status: "validation_error",
        });

        return NextResponse.json(
          { success: false, error: "Name and numeric value are required" },
          { status: 400 }
        );
      }

      // Process the data
      const result = await processData({ name, value });

      const traceContext = getTraceContext();

      logger.info("api.request.completed", {
        route: "/api/demo",
        method: "POST",
        item_name: name,
        duration_ms: Date.now() - startTime,
        status: "success",
      });

      requestDuration.record(Date.now() - startTime, {
        method: "POST",
        route: "/api/demo",
        status_code: "201",
      });
      requestCounter.add(1, {
        method: "POST",
        route: "/api/demo",
        status: "success",
      });

      return NextResponse.json(
        {
          success: true,
          data: { name, value, ...result },
          _trace: {
            traceId: traceContext.traceId,
            spanId: traceContext.spanId,
          },
        },
        {
          status: 201,
          headers: {
            "X-Trace-Id": traceContext.traceId || "",
          },
        }
      );
    } catch (error) {
      span.recordException(error as Error);
      span.setStatus({ code: SpanStatusCode.ERROR });

      logger.error("api.request.failed", {
        route: "/api/demo",
        method: "POST",
        error_type: (error as Error).name,
        error_message: (error as Error).message,
        duration_ms: Date.now() - startTime,
      });

      requestDuration.record(Date.now() - startTime, {
        method: "POST",
        route: "/api/demo",
        status_code: "500",
      });
      requestCounter.add(1, {
        method: "POST",
        route: "/api/demo",
        status: "error",
      });

      return NextResponse.json(
        { success: false, error: "Internal server error" },
        { status: 500 }
      );
    } finally {
      activeRequests.add(-1);
      span.end();
    }
  });
}
