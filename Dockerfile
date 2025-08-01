# builder stage
FROM golang:1.21-alpine AS builder
RUN apk add --no-cache git
WORKDIR /src
COPY . .
RUN go mod download
RUN mkdir -p /out
RUN CGO_ENABLED=0 go build -o /out/backup-agent ./cmd/backup-agent
RUN CGO_ENABLED=0 go build -o /out/restore-agent ./cmd/backup-agent-restore
RUN CGO_ENABLED=0 go build -o /out/peerctl ./cmd/peerctl

# final runtime
FROM alpine:3.18
RUN apk add --no-cache ca-certificates
WORKDIR /data
COPY --from=builder /out/backup-agent /usr/local/bin/backup-agent
COPY --from=builder /out/restore-agent /usr/local/bin/restore-agent
COPY --from=builder /out/peerctl /usr/local/bin/peerctl
# default config mount expected at /app/config.yaml
ENTRYPOINT ["backup-agent", "daemon"]
