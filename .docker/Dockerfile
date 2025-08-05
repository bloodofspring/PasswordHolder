# syntax=docker/dockerfile:1.4

# --- Build stage ---
FROM golang:1.23-alpine AS builder
WORKDIR /app
COPY ./go.mod ./go.sum ./
RUN go mod download
COPY ./ .
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bot run.go

# --- Production image ---
FROM alpine:3.19 AS prod
WORKDIR /app
COPY --from=builder /app/bot ./bot
# В продакшн-образ .env не копируем! Только пример для локальной разработки
COPY ./env.example ./env.example
# Все секреты и переменные окружения должны задаваться через ENV или docker-compose, а не через .env
ENV GIN_MODE=release
HEALTHCHECK --interval=30s --timeout=5s --start-period=5s CMD ["/app/bot", "--healthcheck"]
ENTRYPOINT ["/app/bot"]

# --- Development image ---
FROM golang:1.23-alpine AS dev
WORKDIR /app
COPY ./go.mod ./go.sum ./
RUN go mod download
COPY ./ .
CMD ["go", "run", "run.go"] 