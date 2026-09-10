# The firmware build agent.
#
# Holds the docker socket so ridgelined does not have to: it asks ridgelined for
# work over the compose network, launches the builder image as a SIBLING
# container, and listens on nothing itself. Compromising it gets you the host;
# compromising ridgelined does not get you this.
#
# Ships the docker CLI only — the daemon is the host's, reached through the
# mounted socket.
FROM golang:1.25-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/fwagent ./cmd/fwagent

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends \
      docker.io ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY --from=build /out/fwagent /usr/local/bin/fwagent
ENTRYPOINT ["/usr/local/bin/fwagent"]
