# Ruby project assembly fixture

Create the initializer:

```ruby
# config/initializers/opentelemetry.rb
require 'opentelemetry/sdk'
require 'opentelemetry/instrumentation/all'

OpenTelemetry::SDK.configure do |c|
  c.service_name = 'checkout'
  c.use 'OpenTelemetry::Instrumentation::PG', { db_statement: :obfuscate }
end

at_exit do
  OpenTelemetry.tracer_provider.shutdown
  OpenTelemetry.meter_provider.shutdown if OpenTelemetry.respond_to?(:meter_provider)
end
```

Enrich the active span; `order` and `process_order` are application state the example does not define:

```ruby
require 'opentelemetry/sdk'
require 'semantic_logger'

SemanticLogger.add_appender(io: $stdout, formatter: :json)
logger = SemanticLogger['OrderService']

span = OpenTelemetry::Trace.current_span
span.set_attribute('enduser.id', 'TEST-0001')
span.set_attribute('server.address', 'shop.example.test')

begin
  process_order(order)
rescue StandardError => e
  span.status = OpenTelemetry::Trace::Status.error("#{e.class}: #{e.message}")
  logger.error('order.failed', exception: e, order_id: order.id)
end
```

A snippet without a require is a fragment and is not checked:

```ruby
span.set_attribute('order.id', order.id)
```
