# Stage 1: Build the binary
FROM golang:1.25-alpine AS builder

WORKDIR /app

# Copy go.mod and go.sum first for better layer caching
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the binary with optimisations
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o server ./

# Stage 2: Create a tiny runtime image
FROM alpine:3.19

# Install ca-certificates (useful for HTTPS calls)
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from the builder stage
COPY --from=builder /app/server .

# Expose the port your app listens on (change if needed)
EXPOSE 8080

# Run the binary
CMD ["./server"]