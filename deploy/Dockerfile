# Первая стадия: сборка приложения
FROM golang:1.27.1-alpine AS builder

WORKDIR /src

# Сначала копируем описание зависимостей
COPY go.mod go.sum ./

# Скачиваем зависимости проекта
RUN go mod download

# Копируем исходный код, необходимый для сборки
COPY cmd ./cmd
COPY internal ./internal

# Собираем статический Linux-бинарник
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -buildvcs=false \
    -ldflags="-s -w" \
    -o /out/trip-service \
    ./cmd/trip-service


# Вторая стадия: итоговый образ
FROM scratch

# Запускаем приложение не от root
USER 65532:65532

# Копируем из первой стадии только готовую программу
COPY --from=builder /out/trip-service /trip-service

EXPOSE 8080

ENTRYPOINT ["/trip-service"]