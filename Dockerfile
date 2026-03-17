# ── Build stage ────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

# CGO is required for go-sqlite3
RUN apk add --no-cache gcc musl-dev

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=1 go build -ldflags="-s -w" -o /bin/mock-lipila .

# ── Runtime stage ─────────────────────────────────────────
FROM alpine:3.21

RUN apk add --no-cache ca-certificates

# Non-root user for security
RUN adduser -D -h /app appuser
WORKDIR /app

COPY --from=builder /bin/mock-lipila .

# SQLite database will live in a volume
RUN mkdir -p /app/data && chown appuser:appuser /app/data
VOLUME /app/data

USER appuser

ENV PORT=8080
ENV DB_PATH=/app/data/lipila.db

EXPOSE 8080

ENTRYPOINT ["./mock-lipila"]
