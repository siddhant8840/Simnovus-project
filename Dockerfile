# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy dependency specifications
COPY go.mod ./
# Download any dependencies (none required for standard library, but good practice)
RUN go mod download

# Copy source code
COPY . .

# Build statically linked binaries for both server and simulator
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/simulator ./cmd/simulator

# Minimal runtime stage
FROM alpine:3.20

# Add non-root user for security
RUN adduser -D -g '' appuser

WORKDIR /app
COPY --from=builder /bin/server /app/server
COPY --from=builder /bin/simulator /app/simulator

USER appuser

EXPOSE 8080

ENV PORT=8080
ENV HEARTBEAT_TIMEOUT=30s

CMD ["/app/server"]
