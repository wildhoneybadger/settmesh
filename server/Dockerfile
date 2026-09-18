# --- Build stage ---
FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/settmesh-server .

# --- Runtime stage ---
FROM alpine:3.20

RUN apk add --no-cache ca-certificates && \
    adduser -D -H -u 10001 settmesh && \
    mkdir -p /data && chown settmesh:settmesh /data

WORKDIR /app
COPY --from=builder /out/settmesh-server ./settmesh-server

USER settmesh

EXPOSE 8080
VOLUME ["/data"]

ENTRYPOINT ["./settmesh-server"]
