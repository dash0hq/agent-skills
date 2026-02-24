# Cost Optimization Examples

Real-world scenarios for reducing telemetry costs while maintaining observability.

## Scenario 1: High-Volume API Service

### Problem

```yaml
situation:
  service: REST API
  traffic: 10,000 requests/second
  current_cost: $5,000/month
  target_cost: $500/month

issues:
  - Tracing every request
  - High cardinality from user IDs in attributes
  - Debug logs in production
  - Health checks creating noise
```

### Solution

#### Step 1: Filter Noise at Collector

```yaml
processors:
  filter/noise:
    traces:
      span:
        # Drop health checks (30% of traffic)
        - 'attributes["http.route"] in ["/health", "/ready", "/live"]'
        # Drop synthetic monitoring
        - 'attributes["http.user_agent"] contains "synthetic"'
        - 'attributes["http.user_agent"] contains "pingdom"'
        - 'attributes["http.user_agent"] contains "datadog"'
```

**Impact: -30% trace volume**

#### Step 2: Implement Tail Sampling

```yaml
processors:
  tail_sampling:
    decision_wait: 10s
    policies:
      # Always keep errors
      - name: errors
        type: status_code
        status_code:
          status_codes: [ERROR]

      # Always keep slow requests
      - name: slow
        type: latency
        latency:
          threshold_ms: 500

      # Sample 5% of remainder
      - name: probabilistic
        type: probabilistic
        probabilistic:
          sampling_percentage: 5
```

**Impact: -85% trace volume (from remaining)**

#### Step 3: Remove High-Cardinality Attributes

```yaml
processors:
  attributes/remove:
    actions:
      # Remove unbounded attributes
      - key: user.id
        action: delete
      - key: request.id
        action: delete
      - key: session.id
        action: delete

      # Keep bounded version
      - key: user.tier
        action: upsert
        from_attribute: user.tier
```

**Impact: -20% attribute storage**

#### Step 4: Reduce Log Levels

```javascript
// Before: All logs
logger.level = 'debug';

// After: WARN+ only in production
logger.level = process.env.NODE_ENV === 'production' ? 'warn' : 'debug';
```

**Impact: -90% log volume**

### Result

```yaml
before:
  traces: 10,000/sec × 100% = 10,000/sec
  logs: 50,000 lines/sec
  cost: $5,000/month

after:
  traces: 10,000/sec × 0.7 × 0.15 = 1,050/sec
  logs: 5,000 lines/sec
  cost: ~$500/month

reduction: 90%
```

---

## Scenario 2: Microservices Architecture

### Problem

```yaml
situation:
  services: 50 microservices
  traffic: Variable (10-1000 req/sec per service)
  current_cost: $15,000/month
  target_cost: $3,000/month

issues:
  - All services have same sampling rate
  - High-volume services dominate costs
  - Internal service-to-service calls double-counted
  - Metrics have high cardinality
```

### Solution

#### Step 1: Tiered Sampling by Service Criticality

```yaml
# Tier 1: Critical services (5 services)
critical_services:
  sampling_rate: 0.50  # 50%
  services:
    - payment-service
    - auth-service
    - order-service
    - checkout-service
    - user-service

# Tier 2: Important services (15 services)
important_services:
  sampling_rate: 0.10  # 10%
  services:
    - inventory-service
    - notification-service
    - search-service
    # ... etc

# Tier 3: Support services (30 services)
support_services:
  sampling_rate: 0.01  # 1%
  services:
    - logging-service
    - config-service
    - metrics-service
    # ... etc
```

#### Step 2: Deduplicate Internal Calls

```yaml
processors:
  filter/internal:
    traces:
      span:
        # Drop client spans for internal calls (server span is enough)
        - 'kind == SPAN_KIND_CLIENT and attributes["peer.service"] in internal_services'
```

#### Step 3: Metric Cardinality Control

```yaml
processors:
  metricstransform:
    transforms:
      # Aggregate by route, not full URL
      - include: http.server.duration
        action: update
        operations:
          - action: aggregate_labels
            label_set: [http.method, http.route, service.name]
            aggregation_type: sum
```

#### Step 4: Environment-Based Policies

```yaml
# Production: Strict
production:
  traces:
    default_sample_rate: 0.05
    error_sample_rate: 1.0
  logs:
    level: warn
  metrics:
    collection_interval: 60s

# Staging: Moderate
staging:
  traces:
    default_sample_rate: 0.25
  logs:
    level: info
  metrics:
    collection_interval: 30s
```

### Result

```yaml
before:
  traces: 500,000/min (uniform sampling)
  metrics: 100,000 series
  cost: $15,000/month

after:
  traces: 75,000/min (tiered sampling)
  metrics: 20,000 series (cardinality control)
  cost: ~$3,000/month

reduction: 80%
```

---

## Scenario 3: E-Commerce Platform (Black Friday)

### Problem

```yaml
situation:
  normal_traffic: 1,000 req/sec
  peak_traffic: 50,000 req/sec (Black Friday)
  budget: Fixed at $2,000/month

issues:
  - Cost spikes during sales events
  - Can't increase budget
  - Need to maintain visibility during peak
  - SLOs must be preserved
```

### Solution

#### Step 1: Define SLO-Protected Metrics

```yaml
# These metrics NEVER get sampled
slo_metrics:
  - http.server.duration (for latency SLO)
  - http.server.errors (for error rate SLO)
  - checkout.success_rate (business SLO)

# Use pre-aggregated metrics, not traces
processors:
  spanmetrics:
    metrics_exporter: prometheus
    dimensions:
      - http.method
      - http.route
      - http.status_code
```

#### Step 2: Adaptive Sampling Based on Traffic

```javascript
// Dynamic sampling rate based on current traffic
function calculateSampleRate(currentRps) {
  const TARGET_TRACES_PER_SECOND = 100;

  if (currentRps <= 100) return 1.0;      // Low traffic: keep all
  if (currentRps <= 1000) return 0.1;     // Normal: 10%
  if (currentRps <= 10000) return 0.01;   // High: 1%
  return 0.001;                            // Peak: 0.1%
}

// Apply via feature flag or config
const sampleRate = calculateSampleRate(getCurrentRps());
```

#### Step 3: Prioritize Business-Critical Traces

```yaml
processors:
  tail_sampling:
    policies:
      # Priority 1: Errors (always keep)
      - name: errors
        type: status_code
        status_code: {status_codes: [ERROR]}

      # Priority 2: Purchases (business critical)
      - name: purchases
        type: string_attribute
        string_attribute:
          key: transaction.type
          values: [purchase, checkout, payment]

      # Priority 3: High-value customers
      - name: premium_customers
        type: string_attribute
        string_attribute:
          key: customer.tier
          values: [premium, enterprise]

      # Everything else: aggressive sampling
      - name: default
        type: probabilistic
        probabilistic:
          sampling_percentage: 0.1
```

#### Step 4: Pre-Event Preparation

```yaml
# Week before Black Friday
preparation:
  - Review and tune sampling rates
  - Pre-warm collector capacity
  - Set up cost alerts
  - Document rollback procedures

# During event
monitoring:
  - Watch collector queue depth
  - Monitor export error rates
  - Track cost in real-time

# Emergency procedures
if_over_budget:
  - Increase sampling rate
  - Drop DEBUG/INFO logs
  - Reduce histogram buckets
```

### Result

```yaml
normal_period:
  traffic: 1,000 req/sec
  sample_rate: 10%
  traces: 100/sec
  cost: $1,500/month

peak_period (Black Friday):
  traffic: 50,000 req/sec
  sample_rate: 0.1%
  traces: 50/sec
  cost: Stays within $2,000/month

key_achievement:
  - SLO metrics preserved at 100%
  - All errors captured
  - Business transactions prioritized
  - Fixed budget maintained
```

---

## Scenario 4: Startup on a Tight Budget

### Problem

```yaml
situation:
  stage: Early startup
  budget: $100/month total
  services: 3 (API, Worker, Web)
  traffic: 100 req/sec

needs:
  - Basic observability
  - Error tracking
  - Performance baseline
  - Can't afford comprehensive tracing
```

### Solution

#### Step 1: Metrics-First Approach

```javascript
// Focus on golden signals via metrics (cheap)
const requestCounter = meter.createCounter('http.requests');
const requestDuration = meter.createHistogram('http.duration');
const errorCounter = meter.createCounter('http.errors');

// Minimal cardinality
app.use((req, res, next) => {
  const start = Date.now();
  res.on('finish', () => {
    const labels = {
      method: req.method,
      route: req.route?.path || 'unknown',
      status: Math.floor(res.statusCode / 100) + 'xx'  // Bucket status
    };

    requestCounter.add(1, labels);
    requestDuration.record(Date.now() - start, labels);

    if (res.statusCode >= 500) {
      errorCounter.add(1, labels);
    }
  });
  next();
});
```

#### Step 2: Error-Only Tracing

```yaml
# SDK configuration
sampler:
  type: custom
  config:
    # Only trace errors
    default: 0  # No sampling for success
    error: 1.0  # 100% for errors

# Or use tail sampling at collector
processors:
  tail_sampling:
    policies:
      - name: errors-only
        type: status_code
        status_code:
          status_codes: [ERROR]
```

#### Step 3: Local Debugging, Remote Errors

```javascript
// Development: Full traces to console
if (process.env.NODE_ENV === 'development') {
  exporter = new ConsoleSpanExporter();
  sampler = new AlwaysOnSampler();
}

// Production: Errors only to Dash0
if (process.env.NODE_ENV === 'production') {
  exporter = new OTLPTraceExporter({
    url: process.env.DASH0_ENDPOINT,
    headers: {
      'Authorization': `Bearer ${process.env.DASH0_AUTH_TOKEN}`
    }
  });
  sampler = new ErrorOnlySampler();
}
```

#### Step 4: Free Tier Maximization

```yaml
# Use free tiers strategically
free_options:
  - Dash0 Free Trial: Full features during trial
  - Grafana Cloud Free: 10k series, 50GB logs
  - Honeycomb Free: 20M events/month

strategy:
  metrics: Grafana Cloud Free (sufficient for golden signals)
  traces: Dash0 (errors only, sampled)
  logs: Local files + Grafana Loki (free tier)
```

### Result

```yaml
cost_breakdown:
  metrics: $0 (Grafana free tier)
  traces: $0-50 (Dash0 with aggressive sampling)
  logs: $0 (Loki free tier)
  total: $0-50/month

capabilities:
  - Golden signals monitoring ✓
  - Error tracking with traces ✓
  - Basic alerting ✓
  - Performance baseline ✓

trade_offs:
  - No trace sampling for successful requests
  - Limited historical data
  - Manual scaling of self-hosted components
```

---

## Cost Calculation Formulas

### Traces

```
Monthly cost =
  (requests/sec) ×
  (sample_rate) ×
  (seconds/month) ×
  (avg_spans_per_trace) ×
  (cost_per_span)

Example:
  1,000 req/sec ×
  0.10 (10% sampling) ×
  2,592,000 sec/month ×
  5 spans ×
  $0.000001 = $1,296/month
```

### Metrics

```
Monthly cost =
  (unique_series) ×
  (cost_per_series/month)

Example:
  10,000 series ×
  $0.10/series = $1,000/month
```

### Logs

```
Monthly cost =
  (GB_ingested/month) ×
  (cost_per_GB) +
  (GB_stored) ×
  (storage_cost_per_GB)

Example:
  100 GB × $0.50 +
  500 GB × $0.03 = $65/month
```

### ROI of Reduction

```
Before implementing optimization:
  Traces: $5,000
  Metrics: $2,000
  Logs: $1,000
  Total: $8,000/month

After optimization:
  Traces: $500 (90% reduction via sampling)
  Metrics: $400 (80% reduction via cardinality)
  Logs: $100 (90% reduction via level filtering)
  Total: $1,000/month

Savings: $7,000/month = $84,000/year
```
