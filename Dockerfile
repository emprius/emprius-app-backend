FROM golang:alpine AS builder

# Install deps
RUN apk update && apk add --no-cache build-base

# Set working directory
WORKDIR /src

# Copy only go.mod and go.sum first (they rarely change)
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the code
COPY . .

# Build the binary
RUN go build -o=empriusbackend -ldflags="-s -w" cmd/main.go

# Final minimal image
FROM alpine:latest

WORKDIR /app
COPY --from=builder /src/empriusbackend ./
ENTRYPOINT ["/app/empriusbackend"]
