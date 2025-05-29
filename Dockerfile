FROM golang:1.23.3 AS builder

WORKDIR /app

# Copy only the go.mod and go.sum files to leverage caching.
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /dist/server ./cmd/server

# Use a minimal alpine image for the final stage
FROM alpine:latest

WORKDIR /app

RUN apk --no-cache add ca-certificates

# Copy the binary from builder
COPY --from=builder /dist/server .

# Expose the port
EXPOSE 8080

# Run the application
CMD ["./server"]
