# Eval fixture: uninstrumented Scala HTTP service (see evals/fixtures/README.md).
# Deterministic build: pinned base image carrying JDK 21, sbt 1.12.14, and
# Scala 3.3.8. The container runs the service via `sbt run` so sbt plugins an
# agent adds while instrumenting (for example sbt-javaagent) take effect at
# start; `sbt update` and `sbt compile` at build time warm the dependency and
# compilation caches for the network-restricted runtime.
#
#   docker build -t scala-service-eval .
FROM sbtscala/scala-sbt:eclipse-temurin-21.0.11_10_1.12.14_3.3.8
WORKDIR /app
COPY project ./project
COPY build.sbt ./
RUN sbt update
COPY . .
RUN sbt compile
EXPOSE 8080
CMD ["sbt", "run"]
