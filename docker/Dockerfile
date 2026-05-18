# Multi-stage build — два образа в одном файле:
# Stage 1 "builder": компилируем Go бинарник (большой образ ~800MB с Go toolchain)
# Stage 2 "runner":  запускаем только бинарник (минимальный образ ~10MB)
# Финальный образ не содержит Go компилятор, исходники, go.mod — только бинарник.

# --- Stage 1: Build ---
# FROM golang:1.25-alpine AS builder
#
# Установить системные зависимости
# RUN apk add --no-cache git ca-certificates tzdata
#
# WORKDIR /app
#
# Сначала копируем go.mod/go.sum и скачиваем зависимости.
# Docker кэширует этот слой — если go.mod не изменился, зависимости не перескачиваются.
# COPY go.mod go.sum ./
# RUN go mod download
#
# Копируем исходники и компилируем
# COPY . .
#
# CGO_ENABLED=0 — статическая линковка (нужно для distroless образа)
# -ldflags="-s -w" — убрать debug символы (~30% меньше бинарник)
# RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /app/server ./cmd/server

# --- Stage 2: Run ---
# FROM gcr.io/distroless/static-debian12
#
# distroless — Google образ без shell, пакетного менеджера.
# Минимальная поверхность атаки: нельзя exec в контейнер.
# Альтернатива: alpine (если нужен shell для дебага).
#
# COPY --from=builder /app/server /server
# COPY --from=builder /app/migration /migration
# COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
#
# USER nonroot:nonroot   <- не запускаем от root
# EXPOSE 8080
# ENTRYPOINT ["/server"]
