# Build the dual-homed OTLP relay as a minimal container image.
#
# Build from the evals/ module root (the Docker build context must contain
# go.mod):
#
#   docker build -f cmd/relay/Dockerfile -t agent-skills-eval-relay .
#
# Run attached to both the internal fixture network and a bridge network with
# host-gateway access, with RELAY_UPSTREAM pointing at the host otelsink.
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /relay ./cmd/relay

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /relay /relay
EXPOSE 4317 4318
ENTRYPOINT ["/relay"]
