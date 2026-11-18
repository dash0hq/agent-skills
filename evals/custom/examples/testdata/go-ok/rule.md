# Go module assembly fixture

Name the telemetry package file:

```go
// telemetry/telemetry.go
package telemetry

import (
	"context"

	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Init installs a tracer provider and returns its shutdown function.
func Init(_ context.Context) func(context.Context) error {
	tp := sdktrace.NewTracerProvider()
	otel.SetTracerProvider(tp)
	return tp.Shutdown
}
```

A first program:

```go
package main

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel"
)

func main() {
	ctx, span := otel.Tracer("checkout").Start(context.Background(), "checkout")
	defer span.End()
	slog.InfoContext(ctx, "order placed", "order_id", "TEST-0001")
}
```

A second program in the same rule file gets its own directory:

```go
package main

import "fmt"

func main() {
	fmt.Println("http://collector.example.test:4318")
}
```

A snippet is a fragment and is not compiled:

```go
span.SetAttributes(attribute.String("order.id", "TEST-0001"))
```

```go
// BAD: a printf verb without its argument
package main

import "fmt"

func main() { fmt.Printf("order %s\n") }
```
