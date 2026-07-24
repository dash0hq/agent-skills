# Build the fixture helper (stub downstream server + traffic probe) as a
# minimal container image.
#
# Build from the evals/ module root (the Docker build context must contain
# go.mod):
#
#   docker build -f cmd/evalhelper/Dockerfile -t agent-skills-eval-helper .
#
# Run attached to the internal fixture network, as the stub downstream server
# ("serve") or as the traffic driver ("probe <url> [count]").
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /evalhelper ./cmd/evalhelper

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /evalhelper /evalhelper
EXPOSE 9090
ENTRYPOINT ["/evalhelper"]
