# Python project assembly fixture

Create the telemetry helper:

```python
# app/telemetry.py
from opentelemetry import trace
from opentelemetry.trace import Tracer


def get_tracer(name: str = "checkout") -> Tracer:
    return trace.get_tracer(name)
```

Use it from a handler; `order` is application state the example does not define:

```python
from opentelemetry.trace import StatusCode

from app.telemetry import get_tracer


def place_order():
    with get_tracer().start_as_current_span("order.place") as span:
        span.set_attribute("order.id", order["id"])
        span.set_attribute("server.address", "shop.example.test")
        span.set_status(StatusCode.OK)
```

Enrich the active span:

```python
from opentelemetry import trace

span = trace.get_current_span()
span.set_attribute("enduser.id", "TEST-0001")
```

A snippet without imports is a fragment and is not checked:

```python
span.set_attribute("order.id", order_id)
```
