# ---- Build Stage ----
FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates

WORKDIR /app

# Cache dependencies first
COPY go.mod go.sum ./
RUN go mod download

# Copy the full source tree
COPY . .

# Build a static binary
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /bootstrapnode ./cmd/bootstrapnode

# ---- Runtime Stage ----
FROM alpine:3.21

RUN apk add --no-cache ca-certificates

# Create a non-root user
RUN addgroup -S appgroup && adduser -S appuser -G appgroup

WORKDIR /app

COPY --from=builder /bootstrapnode /app/bootstrapnode

# Data directory for the identity key file
RUN mkdir -p /app/data && chown -R appuser:appgroup /app/data
VOLUME ["/app/data"]

USER appuser

# Expose TCP and UDP (QUIC) ports
EXPOSE 4001/tcp
EXPOSE 4001/udp

ENTRYPOINT ["/app/bootstrapnode"]
