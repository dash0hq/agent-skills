# Kubernetes eval workload: the checkout service pre-instrumented with the
# OpenTelemetry SDK, configured entirely through OTEL_* environment variables
# (see main.go). Deterministic build: pinned base images, CGO disabled,
# static binary.
#
#   docker build -t instrumented-go-service-eval:local .
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /instrumented-go-service .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /instrumented-go-service /instrumented-go-service
EXPOSE 8080
ENTRYPOINT ["/instrumented-go-service"]
