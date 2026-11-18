# Broken Scala project fixture

```scala
import io.opentelemetry.api.trace.Span

object Tagging {
  def tag(): Unit = Span.current().setAttribute("enduser.id", "TEST-0001")
}
```

```scala
import io.opentelemetry.api.trace.{Span, StatusCode}

object Failing {
  def fail(): Unit = Span.current().setStatus(StatusCode.FAILED)
}
```

```scala
import zio.ZIO

object Effects {
  val done = ZIO.unit
}
```
