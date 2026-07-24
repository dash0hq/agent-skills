# Eval fixture: uninstrumented Java HTTP service (see evals/fixtures/README.md).
# Spring Boot Web application; the spring-boot-maven-plugin repackage produces
# the runnable fat jar. Pinned base images for a deterministic build.
#
#   docker build -t java-service-eval .
FROM maven:3.9-eclipse-temurin-21 AS build
WORKDIR /src
COPY pom.xml ./
RUN mvn -q -B dependency:go-offline
COPY src ./src
RUN mvn -q -B package

FROM eclipse-temurin:21-jre
WORKDIR /app
COPY --from=build /src/target/java-service.jar /app/java-service.jar
EXPOSE 8080
ENTRYPOINT ["java", "-jar", "/app/java-service.jar"]
