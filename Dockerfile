# Build stage
FROM golang:alpine AS builder

WORKDIR /app

# Copy go.mod and dependencies
COPY go.mod ./
RUN go mod download

# Copy application source code
COPY . .

# Build a statically linked Go binary
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o toy-blockchain main.go

# Final stage
FROM alpine:latest

WORKDIR /app

# Copy compiled static binary from builder stage
COPY --from=builder /app/toy-blockchain .

# Expose server ports
EXPOSE 8001 8002 8003

# Run the node binary
ENTRYPOINT ["./toy-blockchain"]
