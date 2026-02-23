# Stage 1 — CSS builder
FROM node:20-alpine AS css-builder
WORKDIR /build
COPY package.json package-lock.json ./
RUN npm ci
COPY tailwind.config.js postcss.config.js* ./
COPY static/ static/
COPY web/ web/
RUN npm run build

# Stage 2 — Go builder
FROM golang:1.24-alpine AS go-builder
ARG VERSION=dev
ARG COMMIT=unknown
ARG BRANCH=unknown
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=css-builder /build/web/static/css/app.css web/static/css/app.css
RUN PKG="github.com/joestump/joe-links/internal/build" && \
    CGO_ENABLED=0 go build \
      -ldflags="-s -w -X ${PKG}.Version=${VERSION} -X ${PKG}.Commit=${COMMIT} -X ${PKG}.Branch=${BRANCH}" \
      -o joe-links ./cmd/joe-links

# Stage 3 — final
FROM alpine:latest
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=go-builder /build/joe-links .
EXPOSE 8080
CMD ["./joe-links", "serve"]
