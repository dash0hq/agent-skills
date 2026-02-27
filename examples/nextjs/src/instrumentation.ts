import { NodeSDK } from '@opentelemetry/sdk-node';
import { getNodeAutoInstrumentations } from '@opentelemetry/auto-instrumentations-node';
import { OTLPTraceExporter } from '@opentelemetry/exporter-trace-otlp-http';
import { OTLPMetricExporter } from '@opentelemetry/exporter-metrics-otlp-http';
import { OTLPLogExporter } from '@opentelemetry/exporter-logs-otlp-http';
import { PeriodicExportingMetricReader } from '@opentelemetry/sdk-metrics';
import {
  BatchLogRecordProcessor,
  LoggerProvider,
} from '@opentelemetry/sdk-logs';
import { logs } from '@opentelemetry/api-logs';
import { resourceFromAttributes } from '@opentelemetry/resources';
import {
  ATTR_SERVICE_NAME,
  ATTR_SERVICE_VERSION,
} from '@opentelemetry/semantic-conventions';

export async function register() {
  if (process.env.NEXT_RUNTIME === 'nodejs') {
    // Read env vars inside register() to ensure .env.local is loaded
    const OTEL_ENDPOINT =
      process.env.OTEL_EXPORTER_OTLP_ENDPOINT || 'http://localhost:4318';
    const OTEL_AUTH_TOKEN = process.env.NEXT_PUBLIC_OTEL_AUTH_TOKEN;

    console.log('[OTel] Endpoint:', OTEL_ENDPOINT);
    console.log('[OTel] Auth:', OTEL_AUTH_TOKEN ? 'configured' : 'missing');

    const exporterHeaders: Record<string, string> = {};
    if (OTEL_AUTH_TOKEN) {
      exporterHeaders['Authorization'] = `Bearer ${OTEL_AUTH_TOKEN}`;
    }

    const resource = resourceFromAttributes({
      [ATTR_SERVICE_NAME]: process.env.OTEL_SERVICE_NAME || 'nextjs-demo',
      [ATTR_SERVICE_VERSION]: '1.0.0',
      'deployment.environment': process.env.NODE_ENV || 'development',
    });

    // Initialize LoggerProvider for structured logging
    const logExporter = new OTLPLogExporter({
      url: `${OTEL_ENDPOINT}/v1/logs`,
      headers: exporterHeaders,
    });

    const loggerProvider = new LoggerProvider({
      resource,
      processors: [new BatchLogRecordProcessor(logExporter)],
    });
    logs.setGlobalLoggerProvider(loggerProvider);

    // Initialize NodeSDK for traces and metrics
    const sdk = new NodeSDK({
      resource,
      traceExporter: new OTLPTraceExporter({
        url: `${OTEL_ENDPOINT}/v1/traces`,
        headers: exporterHeaders,
      }),
      metricReader: new PeriodicExportingMetricReader({
        exporter: new OTLPMetricExporter({
          url: `${OTEL_ENDPOINT}/v1/metrics`,
          headers: exporterHeaders,
        }),
        exportIntervalMillis: 10000,
      }),
      instrumentations: [
        getNodeAutoInstrumentations({
          '@opentelemetry/instrumentation-fs': { enabled: false },
          '@opentelemetry/instrumentation-dns': { enabled: false },
        }),
      ],
    });

    sdk.start();

    // Graceful shutdown
    process.on('SIGTERM', () => {
      sdk
        .shutdown()
        .then(() => loggerProvider.shutdown())
        .then(() => console.log('Telemetry shut down'))
        .catch((err) => console.error('Shutdown error', err))
        .finally(() => process.exit(0));
    });
  }
}
