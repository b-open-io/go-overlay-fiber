# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies for CGO (required for SQLite)
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Set working directory
WORKDIR /build

# Copy go-sdk dependency
COPY go-sdk ./go-sdk

# Copy go-overlay-fiber
COPY go-overlay-fiber ./go-overlay-fiber

# Set working directory to go-overlay-fiber
WORKDIR /build/go-overlay-fiber

# Download dependencies
RUN go mod download

# Build the application
RUN CGO_ENABLED=1 go build -o overlay-server ./example

# Runtime stage
FROM alpine:latest

# Install required libraries for SQLite
RUN apk --no-cache add ca-certificates sqlite-libs

# Recreate the build directory structure to match runtime.Caller paths
WORKDIR /build/go-overlay-fiber

# Copy the binary from builder
COPY --from=builder /build/go-overlay-fiber/overlay-server .

# Copy template files needed by the application
COPY --from=builder /build/go-overlay-fiber/pkg/server/templates ./pkg/server/templates

# Expose the application port
EXPOSE 8080

# Start the application
CMD ["./overlay-server"]
