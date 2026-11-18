# Scala project assembly fixture

Create the telemetry helper in the root project:

```scala
// src/main/scala/example/Telemetry.scala
package example

import io.opentelemetry.api.GlobalOpenTelemetry
import io.opentelemetry.api.trace.Tracer

object Telemetry {
  val tracer: Tracer = GlobalOpenTelemetry.getTracer("checkout")
}
```

Use it from a second file of the same project; `Order` and `saveOrder` are application state the example does not define:

```scala
// src/main/scala/example/Checkout.scala
package example

import io.opentelemetry.api.trace.StatusCode

object Checkout {
  def place(order: Order): Unit = {
    val span = Telemetry.tracer.spanBuilder("order.place").startSpan()
    try {
      span.setAttribute("enduser.id", "TEST-0001")
      span.setAttribute("server.address", "shop.example.test")
      saveOrder(order)
      span.setStatus(StatusCode.OK)
    } finally {
      span.end()
    }
  }
}
```

A standalone block compiles in a subproject of its own:

```scala
import com.typesafe.scalalogging.LazyLogging
import io.opentelemetry.api.trace.Span

object OrderService extends LazyLogging {
  def tag(): Unit = {
    Span.current().setAttribute("order.id", "TEST-0002")
    logger.info("order.tagged")
  }
}
```

A second standalone block may declare the same object:

```scala
import org.slf4j.LoggerFactory

object OrderService {
  private val logger = LoggerFactory.getLogger(getClass)

  def fail(e: Exception): Unit =
    logger.atError().addKeyValue("exception.type", e.getClass.getName).log("order.failed")
}
```

A snippet without a top-level definition is a fragment and is not compiled:

```scala
import io.opentelemetry.api.trace.Span

val span = Span.current()
```

An sbt build definition is a fragment:

```scala
// build.sbt
libraryDependencies += "io.opentelemetry" % "opentelemetry-api" % "<version>"
```
