# Build stage
FROM golang:1.24-alpine AS builder

# Install build dependencies for CGO (required for SQLite)
RUN apk add --no-cache gcc musl-dev sqlite-dev

# Set working directory
WORKDIR /app

# Copy go mod files from all required repos
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy all source code
COPY . ./

# Build the application
RUN CGO_ENABLED=1 go build -o overlay-server ./example

# Runtime stage
FROM alpine:latest

# Install required libraries for SQLite
RUN apk --no-cache add ca-certificates sqlite-libs

# Set working directory
WORKDIR /app

# Copy the binary from builder
COPY --from=builder /app/overlay-server .

# Copy template files needed by the application
COPY --from=builder /app/pkg/server/templates ./pkg/server/templates

# Expose the application port
EXPOSE 8080

# Start the application
CMD ["./overlay-server"]
