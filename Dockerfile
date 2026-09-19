# Build stage
FROM golang:1.26-alpine AS builder

WORKDIR /build

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Build binaries
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags "-w -s" -o bin/analytic cmd/analytic/main.go && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-w -s" -o bin/migrator cmd/migrator/main.go && \
    CGO_ENABLED=0 GOOS=linux go build -ldflags "-w -s" -o bin/worker cmd/worker/main.go

# Runtime stage
FROM alpine:latest

# Install ca-certificates, netcat (for health checks) and apply security updates
RUN apk update && \
    apk --no-cache add ca-certificates netcat-openbsd && \
    rm -rf /var/cache/apk/*

WORKDIR /app

# Copy binaries from builder
COPY --from=builder /build/bin/analytic .
COPY --from=builder /build/bin/migrator .
COPY --from=builder /build/bin/worker .

# Copy migrations directory
COPY ./migrations ./migrations

# Copy web directory (HTML files and static assets)
COPY ./web ./web

# Copy entrypoint script
COPY docker-entrypoint.sh /
RUN chmod +x /docker-entrypoint.sh

# Set entrypoint
ENTRYPOINT ["/docker-entrypoint.sh"]

# Run the application
CMD ["./analytic"]