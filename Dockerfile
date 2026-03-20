# --- Build stage ---
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /loggos .

# --- Runtime stage ---
FROM alpine:3.23

RUN apk add --no-cache ca-certificates

COPY --from=builder /loggos /usr/local/bin/loggos

EXPOSE 8080

ENTRYPOINT ["loggos"]
