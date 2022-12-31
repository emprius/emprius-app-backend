FROM golang:alpine AS builder

WORKDIR /src
COPY . .
RUN apk update && apk add build-base
RUN go build . -o=empriusbackend -ldflags="-s -w"

FROM alpine:latest

WORKDIR /app
COPY --from=builder /src/empriusbackend ./
ENTRYPOINT ["/app/empriusbackend"]
