# Broken Python project fixture

```python
from opentelemetry import trace

trace.get_current_span().set_attribute("enduser.id", "TEST-0001")
```

```python
from opentelemetry import trace
from opentelemetry.trace import StatusCode


def place_order():
    span = trace.get_tracer("checkout").start_span("order.place")
    span.set_status(StatusCode.FAILED)
    span.end()
```
