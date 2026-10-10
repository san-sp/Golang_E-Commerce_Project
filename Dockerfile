# Stage 1: Build the Go application
FROM golang:1.27.1-alpine AS builder

WORKDIR /src

# Copy dependency manifests first for better build caching.
COPY go.mod go.sum ./
RUN go mod download

# Copy the project source.
COPY . .

# Select which application to build.
ARG APP=api

# Compile a static Linux executable.
RUN CGO_ENABLED=0 GOOS=linux \
    go build -trimpath -ldflags="-s -w" \
    -o /out/app ./cmd/${APP}


# Stage 2: Run the compiled application
FROM alpine:3.22

RUN apk add --no-cache ca-certificates tzdata \
    && addgroup -S app \
    && adduser -S -G app app

WORKDIR /app

COPY --from=builder --chown=app:app /out/app /app/app

USER app

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["/app/app"]
