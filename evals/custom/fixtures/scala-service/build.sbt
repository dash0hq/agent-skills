// Eval fixture: deliberately uninstrumented Scala HTTP service (see
// evals/fixtures/README.md for the contract). Zero library dependencies: the
// server is com.sun.net.httpserver and the outbound call uses
// java.net.http.HttpClient. The container runs the service via `sbt run`, and
// `run / fork := true` starts it in a fresh JVM (required for any -javaagent
// wiring an agent adds while instrumenting).
ThisBuild / scalaVersion := "3.8.4"

lazy val root = (project in file("."))
  .settings(
    name := "scala-service",
    run / fork := true,
    run / connectInput := false,
    outputStrategy := Some(StdoutOutput)
  )
