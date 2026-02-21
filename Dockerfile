# Stage 1 — builder
FROM golang:1.24-alpine AS builder
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o joe-links ./cmd/joe-links

# Stage 2 — final
FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=builder /build/joe-links .
EXPOSE 8080
CMD ["./joe-links", "serve"]
