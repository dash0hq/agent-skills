# Eval fixture: uninstrumented Go HTTP service (see evals/fixtures/README.md).
# Deterministic build: pinned base images, CGO disabled, static binary.
#
#   docker build -t go-service-eval .
#
# The go.* glob copies go.sum too once dependencies (for example the ones an
# agent adds while instrumenting) introduce it.
FROM golang:1.24 AS build
WORKDIR /src
COPY go.* ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /go-service .

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /go-service /go-service
EXPOSE 8080
ENTRYPOINT ["/go-service"]
