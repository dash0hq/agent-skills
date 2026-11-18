# PHP project assembly fixture

Create the telemetry helper:

```php
<?php
// src/Telemetry.php

declare(strict_types=1);

namespace App;

use OpenTelemetry\API\Globals;
use OpenTelemetry\API\Trace\TracerInterface;

final class Telemetry
{
    public static function tracer(): TracerInterface
    {
        return Globals::tracerProvider()->getTracer('checkout');
    }
}
```

Enrich the active span; `$order` and `saveOrder()` are application state the example does not define:

```php
use OpenTelemetry\API\Trace\Span;
use OpenTelemetry\API\Trace\StatusCode;

$span = Span::getCurrent();
$span->setAttribute('order.id', $order['id']);
$span->setAttribute('enduser.id', 'TEST-0001');
$span->setAttribute('server.address', 'shop.example.test');
saveOrder($order);
$span->setStatus(StatusCode::STATUS_OK);
```

Log with Monolog:

```php
<?php

use Monolog\Formatter\JsonFormatter;
use Monolog\Handler\StreamHandler;
use Monolog\Level;
use Monolog\Logger;

$handler = new StreamHandler('php://stdout', Level::Info);
$handler->setFormatter(new JsonFormatter());
$logger = new Logger('app');
$logger->pushHandler($handler);
$logger->info('order.placed', ['order_id' => 'TEST-0001']);
```

A snippet without a `use` statement or `<?php` tag is a fragment and is not checked:

```php
$span->setAttribute('order.id', $orderId);
```
