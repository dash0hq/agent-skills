---
title: "Receivers"
impact: CRITICAL
tags:
  - receivers
  - otlp
  - grpc
  - http
  - prometheus
  - filelog
  - hostmetrics
---

# Receivers

Receivers define how telemetry enters the Collector.
Every pipeline must start with at least one receiver.

## Decision table

| Source | Receiver | When to use |
|--------|----------|-------------|
| Instrumented applications (SDKs) | `otlp` | Applications export traces, metrics, or logs via OTLP |
| Prometheus endpoints | `prometheus` | Applications expose `/metrics` endpoints that need scraping |
| Log files on disk | `filelog` | Applications write logs to files instead of stdout |
| System metrics (CPU, memory, disk) | `hostmetrics` | You need infrastructure-level metrics from the host |
| Container logs (Kubernetes) | `filelog` | Collect pod logs from node filesystem paths |

## OTLP receiver

The OTLP receiver is the primary receiver for instrumented applications.
It accepts telemetry over gRPC (port 4317) and HTTP (port 4318).

### Minimal configuration

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318
```

### Bind address

Use `0.0.0.0` when the Collector receives telemetry from other containers or pods.
Use `localhost` only when all senders run on the same host and you want to restrict network access.

```yaml
# GOOD — accessible from other pods in Kubernetes
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317

# BAD — unreachable from other containers
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: localhost:4317
```

### gRPC tuning

Increase `max_recv_msg_size_mib` when applications send large batches (default is 4 MiB).
Configure keepalive to detect dead connections in environments with load balancers or service meshes.

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
        max_recv_msg_size_mib: 16
        keepalive:
          server_parameters:
            max_connection_idle: 60s
            max_connection_age: 300s
            max_connection_age_grace: 60s
          enforcement_policy:
            min_time: 10s
            permit_without_stream: true
```

### TLS

Enable TLS when the Collector is exposed outside a trusted network.
In Kubernetes with a service mesh (Istio, Linkerd), mTLS is handled by the mesh — do not configure TLS on the receiver.

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
        tls:
          cert_file: /certs/tls.crt
          key_file: /certs/tls.key
      http:
        endpoint: 0.0.0.0:4318
        tls:
          cert_file: /certs/tls.crt
          key_file: /certs/tls.key
```

## Prometheus receiver

Use the Prometheus receiver to scrape `/metrics` endpoints from applications that expose Prometheus-format metrics.

### Static targets

```yaml
receivers:
  prometheus:
    config:
      scrape_configs:
        - job_name: "my-service"
          scrape_interval: 30s
          static_configs:
            - targets:
                - "my-service:8080"
```

### Kubernetes service discovery

Use `kubernetes_sd_configs` to discover scrape targets automatically in Kubernetes.
Add `relabel_configs` to filter by annotation (e.g., `prometheus.io/scrape: "true"`).

```yaml
receivers:
  prometheus:
    config:
      scrape_configs:
        - job_name: "kubernetes-pods"
          scrape_interval: 30s
          kubernetes_sd_configs:
            - role: pod
          relabel_configs:
            - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_scrape]
              action: keep
              regex: "true"
            - source_labels: [__meta_kubernetes_pod_annotation_prometheus_io_port]
              action: replace
              target_label: __address__
              regex: (.+)
              replacement: "${1}:${2}"
            - source_labels: [__meta_kubernetes_pod_ip, __meta_kubernetes_pod_annotation_prometheus_io_port]
              action: replace
              target_label: __address__
              regex: "(.+);(.+)"
              replacement: "${1}:${2}"
```

### Metric relabeling

Drop high-cardinality metrics at scrape time to reduce storage costs.

```yaml
receivers:
  prometheus:
    config:
      scrape_configs:
        - job_name: "my-service"
          static_configs:
            - targets: ["my-service:8080"]
          metric_relabel_configs:
            - source_labels: [__name__]
              regex: "go_gc_.*"
              action: drop
```

## Filelog receiver

Use the filelog receiver to collect logs from files on disk.
This is the primary receiver for Kubernetes pod logs when not using a logging agent.

### Basic file collection

```yaml
receivers:
  filelog:
    include:
      - /var/log/myapp/*.log
    exclude:
      - /var/log/myapp/debug.log
    start_at: end
```

Set `start_at: end` to skip existing log lines on first startup.
Set `start_at: beginning` only when you need to ingest historical logs.

### Multiline parsing

Use `multiline` to reassemble stack traces or other multi-line log entries.

```yaml
receivers:
  filelog:
    include:
      - /var/log/myapp/*.log
    multiline:
      line_start_pattern: '^\d{4}-\d{2}-\d{2}'
```

### Severity parsing

Extract severity from log lines to enable filtering by log level.

```yaml
receivers:
  filelog:
    include:
      - /var/log/myapp/*.log
    operators:
      - type: regex_parser
        regex: '^(?P<time>\S+) (?P<severity>\w+) (?P<message>.*)'
        severity:
          parse_from: attributes.severity
        timestamp:
          parse_from: attributes.time
          layout: "%Y-%m-%dT%H:%M:%S.%fZ"
```

### Kubernetes pod logs

Kubernetes writes pod logs to `/var/log/pods/<namespace>_<pod-name>_<pod-uid>/<container-name>/*.log`.
The filelog receiver running in a DaemonSet reads these files from the node filesystem.

For applications to produce well-structured logs that the filelog receiver can parse, see [logs — container runtimes](../../otel-instrumentation/rules/logs.md#container-runtimes).

#### Minimal configuration

```yaml
receivers:
  filelog:
    include:
      - /var/log/pods/*/*/*.log
    exclude:
      - /var/log/pods/*/otc-container/*.log
    start_at: end
    include_file_path: true
    include_file_name: false
    operators:
      - type: container
        id: container-parser
```

The `container` operator auto-detects CRI and Docker log formats.

#### Production configuration with resource attribute extraction

Extract `k8s.namespace.name`, `k8s.pod.name`, `k8s.pod.uid`, and `k8s.container.name` from the log file path.
These attributes enable the `k8sattributes` processor to resolve the full set of Kubernetes metadata (deployment, node, labels) without relying on connection IP.

```yaml
receivers:
  filelog:
    include:
      - /var/log/pods/*/*/*.log
    exclude:
      - /var/log/pods/*/otc-container/*.log
    start_at: end
    include_file_path: true
    include_file_name: false
    operators:
      # Parse CRI/Docker log format
      - type: container
        id: container-parser

      # Extract K8s metadata from file path:
      # /var/log/pods/<namespace>_<pod-name>_<pod-uid>/<container-name>/*.log
      - type: regex_parser
        id: extract-k8s-metadata
        regex: '^\/var\/log\/pods\/(?P<namespace>[^_]+)_(?P<pod_name>[^_]+)_(?P<pod_uid>[^\/]+)\/(?P<container_name>[^\/]+)\/'
        parse_from: attributes["log.file.path"]
        preserve_to: attributes["log.file.path"]

      # Move extracted fields to resource attributes
      - type: move
        from: attributes.namespace
        to: resource["k8s.namespace.name"]
      - type: move
        from: attributes.pod_name
        to: resource["k8s.pod.name"]
      - type: move
        from: attributes.pod_uid
        to: resource["k8s.pod.uid"]
      - type: move
        from: attributes.container_name
        to: resource["k8s.container.name"]
```

Set `include_file_path: true` so that the `log.file.path` attribute is available for the regex parser.
The `preserve_to` setting keeps the original file path attribute after parsing.

#### Excluding Collector logs

Exclude the Collector's own container logs to prevent a feedback loop where the Collector ingests and re-exports its own log output.

```yaml
# GOOD — exclude the collector container
receivers:
  filelog:
    include:
      - /var/log/pods/*/*/*.log
    exclude:
      - /var/log/pods/*/otc-container/*.log

# BAD — no exclusion, Collector logs are re-ingested
receivers:
  filelog:
    include:
      - /var/log/pods/*/*/*.log
```

Replace `otc-container` with the actual container name of the Collector in your DaemonSet manifest.

#### Structured JSON parsing

When applications write structured JSON to stdout (the [recommended approach](../../otel-instrumentation/rules/logs.md#container-runtimes)), add a JSON parser after the container operator to extract structured fields.

```yaml
receivers:
  filelog:
    include:
      - /var/log/pods/*/*/*.log
    exclude:
      - /var/log/pods/*/otc-container/*.log
    start_at: end
    include_file_path: true
    include_file_name: false
    operators:
      - type: container
        id: container-parser
      - type: json_parser
        id: json-parser
        parse_from: body
        timestamp:
          parse_from: attributes.timestamp
          layout: "%Y-%m-%dT%H:%M:%S.%fZ"
        severity:
          parse_from: attributes.level
```

Adjust `parse_from` paths for `timestamp` and `severity` to match the field names your application uses (e.g., `attributes.time`, `attributes.severity`, `attributes.lvl`).

## Hostmetrics receiver

Use the hostmetrics receiver to collect system-level metrics (CPU, memory, disk, network, filesystem).
Deploy this receiver only in agent mode (DaemonSet) where it runs on each node.

### Configuration

```yaml
receivers:
  hostmetrics:
    collection_interval: 30s
    scrapers:
      cpu:
        metrics:
          system.cpu.utilization:
            enabled: true
      memory:
        metrics:
          system.memory.utilization:
            enabled: true
      disk: {}
      network: {}
      filesystem:
        exclude_mount_points:
          mount_points:
            - /dev/*
            - /proc/*
            - /sys/*
          match_type: regexp
```

### Filesystem filtering

Exclude virtual and system filesystems to avoid noisy, unhelpful metrics.

```yaml
receivers:
  hostmetrics:
    collection_interval: 30s
    scrapers:
      filesystem:
        exclude_fs_types:
          fs_types:
            - autofs
            - binfmt_misc
            - cgroup
            - cgroup2
            - configfs
            - debugfs
            - devpts
            - devtmpfs
            - fusectl
            - hugetlbfs
            - mqueue
            - nsfs
            - overlay
            - proc
            - procfs
            - pstore
            - rpc_pipefs
            - securityfs
            - sysfs
            - tmpfs
            - tracefs
          match_type: strict
```

## Anti-patterns

- **Declaring a receiver without adding it to a pipeline.**
  The Collector rejects the configuration at startup.
  Every receiver must appear in at least one pipeline under `service.pipelines`.
- **Using `localhost` as bind address in Kubernetes.**
  Other pods cannot reach the receiver.
  Use `0.0.0.0` to listen on all interfaces.
- **Missing `start_at: end` on filelog in production.**
  The default (`beginning`) causes the Collector to re-ingest all existing log files on restart, creating duplicate logs and a burst of load.
- **Running hostmetrics as a gateway Deployment.**
  Host metrics are per-node.
  A Deployment sees only the metrics of the node it happens to be scheduled on.
  Use a DaemonSet instead.

## References

- [OTLP receiver](https://github.com/open-telemetry/opentelemetry-collector/tree/main/receiver/otlpreceiver)
- [Prometheus receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/prometheusreceiver)
- [Filelog receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/filelogreceiver)
- [Hostmetrics receiver](https://github.com/open-telemetry/opentelemetry-collector-contrib/tree/main/receiver/hostmetricsreceiver)
