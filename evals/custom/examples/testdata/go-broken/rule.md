# Broken Go module fixture

```go
package main

import (
	"context"

	"go.opentelemetry.io/otel"
)

func main() {
	_, span := otel.Tracer("checkout").Start(context.Background(), "checkout")
	span.SetStatus(codes.Failed, "order TEST-0001 failed")
	span.End()
}
```

```go
package main

import "fmt"

func main() {
	fmt.Println("TEST-0002")
}
```
