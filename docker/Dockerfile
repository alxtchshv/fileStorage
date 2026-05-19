FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o server ./cmd/server

FROM gcr.io/distroless/static-debian12

COPY --from=builder /app/server /server
COPY --from=builder /app/migration /migration
COPY --from=builder /app/internal/handler/web /internal/handler/web
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

USER nonroot:nonroot

EXPOSE 8080

ENTRYPOINT ["/server"]
