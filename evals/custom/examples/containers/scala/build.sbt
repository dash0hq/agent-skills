// Pinned libraries the Scala examples in skills/ are compiled against (see
// evals/custom/examples/scala.go). Scala and sbt match the scala-service
// fixture. Dependabot has no sbt ecosystem: bump these by hand to the latest
// release on Maven Central. Next to this file the validator writes examples.sbt,
// which declares the root project and one subproject per standalone example
// block; every project inherits these ThisBuild settings.
ThisBuild / scalaVersion := "3.10.0"

ThisBuild / libraryDependencies ++= Seq(
  "io.opentelemetry" % "opentelemetry-api" % "1.68.0",
  "org.slf4j" % "slf4j-api" % "2.0.20",
  "com.typesafe.scala-logging" %% "scala-logging" % "3.9.6"
)
