# Broken Ruby project fixture

```ruby
require 'opentelemetry/sdk'

OpenTelemetry::Trace.current_span.set_attribute('enduser.id', 'TEST-0001')
```

```ruby
require 'opentelemetry/sdk'

span = OpenTelemetry::Trace.current_span
span.status = OpenTelemetry::Trace::Status.failed('payment declined')
```
