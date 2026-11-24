# Broken PHP project fixture

```php
use OpenTelemetry\API\Trace\Span;

Span::getCurrent()->setAttribute('enduser.id', 'TEST-0001');
```

```php
use OpenTelemetry\API\Trace\Span;
use OpenTelemetry\API\Trace\StatusCode;

$span = Span::getCurrent();
$span->setStatusCode(StatusCode::STATUS_ERROR, 'TEST-0001 failed');
```
