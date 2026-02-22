# ─── Build stage ───────────────────────────────────────────────────────────────
FROM golang:1.24-alpine AS builder

WORKDIR /build

# Cache dependency downloads as a separate layer.
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source and compile a statically-linked binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /server ./cmd/server

# ─── Runtime stage ─────────────────────────────────────────────────────────────
FROM alpine:3.21

# CA certificates are required for HTTPS calls to OAuth and email/SMS providers.
RUN apk --no-cache add ca-certificates wget

# Run as a non-root user.
RUN addgroup -S tusker && adduser -S tusker -G tusker

WORKDIR /app
COPY --from=builder /server /app/server

USER tusker

EXPOSE 8080

HEALTHCHECK --interval=15s --timeout=5s --start-period=10s --retries=3 \
  CMD wget -qO /dev/null http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/server"]
