# --- Stage 1: Build ---
FROM golang:latest as builder

# Set working directory inside the container
WORKDIR /app

# Copy go mod and sum files first (better caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the code
COPY . .

# Build the Go app for Linux and statically link it
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# --- Stage 2: Run ---
FROM alpine:latest

# Install certificate store (for HTTPS calls)
RUN apk --no-cache add ca-certificates

# Set working directory
WORKDIR /root/

# Copy binary from builder
COPY --from=builder /app/main .

# Expose port (Cloud Run expects 8080)
EXPOSE 8080

# Command to run the app
CMD ["./main"]