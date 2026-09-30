# Build the React frontend.
FROM node:22-alpine AS frontend-builder

WORKDIR /frontend
COPY frontend/package.json ./
RUN npm install
COPY frontend/ ./
RUN npm run build

# Build the Go backend.
FROM golang:1.24-alpine AS backend-builder

WORKDIR /app

# Copy dependency specifications
COPY go.mod ./
# Download any dependencies (none required for standard library, but good practice)
RUN go mod download

# Copy source code
COPY . .

# Include the frontend bundle so the Go server can serve it.
COPY --from=frontend-builder /frontend/dist ./frontend/dist

# Build statically linked binaries for both server and simulator
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/server ./cmd/server
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o /bin/simulator ./cmd/simulator

# Minimal runtime stage
FROM alpine:3.20

# Add non-root user for security
RUN adduser -D -g '' appuser

WORKDIR /app
COPY --from=backend-builder /bin/server /app/server
COPY --from=backend-builder /bin/simulator /app/simulator
COPY --from=backend-builder /app/frontend/dist /app/frontend/dist

USER appuser

EXPOSE 8840

ENV PORT=8840
ENV HEARTBEAT_TIMEOUT=30s

CMD ["/app/server"]
