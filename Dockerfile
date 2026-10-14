# Cloud Run (linux/amd64). Thin image, CGO-free (gen2brain/avif via wazero).
FROM golang:1.24.0-alpine3.20 AS builder

ENV CGO_ENABLED=0 \
    GOOS=linux \
    GOARCH=amd64 \
    GOFLAGS="-trimpath -mod=readonly"

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN go build -ldflags="-s -w -buildid=" -o /out/server ./cmd/server \
    && chmod 755 /out/server \
    && test -x /out/server

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata

COPY --from=builder /out/server /usr/local/bin/server
RUN chmod +x /usr/local/bin/server && test -x /usr/local/bin/server

ENV PORT=8080
EXPOSE 8080

USER 65534:65534

CMD ["/usr/local/bin/server"]
