# Invalid Collector configuration fixture

The memory limiter below is missing the required check interval, so
component-level validation fails even though the block is a service-less
fragment.

```yaml
processors:
  memory_limiter:
    limit_mib: 512
```
