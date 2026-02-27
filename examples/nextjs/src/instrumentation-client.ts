import { init, addSignalAttribute, sendEvent } from '@dash0/sdk-web';

// Initialize browser telemetry as early as possible
// For production, use environment variables for endpoint and token
init({
  serviceName: 'nextjs-demo-frontend',
  endpoint: {
    // In production, replace with your actual endpoint and token
    url: process.env.NEXT_PUBLIC_OTEL_ENDPOINT || 'http://localhost:4318',
    // For local development with OTel Collector, use a placeholder token
    // In production, set NEXT_PUBLIC_OTEL_AUTH_TOKEN
    authToken: process.env.NEXT_PUBLIC_OTEL_AUTH_TOKEN || 'dev-token',
  },
  // Enable trace propagation to backend
  propagateTraceHeadersCorsURLs: [
    // Match your API routes
    /\/api\/.*/,
    // Add your backend domains for CORS propagation
    // /api\.yoursite\.com/,
  ],
});

// Add default attributes for all telemetry
addSignalAttribute('app.version', '1.0.0');
addSignalAttribute('app.environment', process.env.NODE_ENV || 'development');

// Export utilities for use in components
export { addSignalAttribute, sendEvent };
