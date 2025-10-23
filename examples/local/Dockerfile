# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies for CGO (required for SQLite)
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Set working directory
WORKDIR /app

# Copy go mod files from all required repos
COPY go-overlay-fiber/go.mod go-overlay-fiber/go.sum ./go-overlay-fiber/
COPY overlay/go.mod overlay/go.sum ./overlay/
COPY go-sdk/go.mod go-sdk/go.sum ./go-sdk/
COPY go-overlay-services/go.mod go-overlay-services/go.sum ./go-overlay-services/
COPY go-overlay-discovery-services/go.mod go-overlay-discovery-services/go.sum ./go-overlay-discovery-services/

# Download dependencies
WORKDIR /app/go-overlay-fiber
RUN go mod download

# Copy all source code
WORKDIR /app
COPY go-overlay-fiber ./go-overlay-fiber
COPY overlay ./overlay
COPY go-sdk ./go-sdk
COPY go-overlay-services ./go-overlay-services
COPY go-overlay-discovery-services ./go-overlay-discovery-services

# Build the application
WORKDIR /app/go-overlay-fiber
RUN CGO_ENABLED=1 go build -o overlay-server ./examples/local

# Runtime stage
FROM alpine:latest

# Install required libraries for SQLite
RUN apk --no-cache add ca-certificates sqlite-libs

# Set working directory
WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/go-overlay-fiber/overlay-server .

# Copy template files needed by the application
COPY --from=builder /app/go-overlay-fiber/pkg/server/templates ./go-overlay-fiber/pkg/server/templates

# Expose the application port
EXPOSE 8080

# Start the application
CMD ["./overlay-server"]
