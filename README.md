# TripGo

HTTP-сервис для создания, получения и завершения поездок. Данные хранятся в
PostgreSQL. HTTP-типы и серверные интерфейсы сгенерированы из OpenAPI-контракта.

## Реализовано в ЛР1

- `POST /api/v1/trips` — создание поездки;
- `GET /api/v1/trips/{tripId}` — получение поездки;
- `POST /api/v1/trips/{tripId}/finish` — завершение поездки;
- `GET /health` и `GET /ready`;
- PostgreSQL через `pgxpool`, SQL через `squirrel`;
- миграции `goose` для `trips` и `trip_status_history`;
- контекстный менеджер транзакций;
- ошибки RFC 9457 в `application/problem+json`;
- graceful shutdown по `SIGINT` и `SIGTERM`;
- дополнительное задание: многостадийная Docker-сборка и запуск в Kubernetes
  рядом с PostgreSQL.

## Требования

- Go 1.27.1;
- GNU Make;
- Docker;
- `tripgoctl` и `kubectl`;
- WSL/Linux shell для команд `Makefile`.

## Конфигурация

`tripgoctl environment start` создаёт локальный `.env`. Пример конфигурации
лежит в `.env.example`.

| Переменная | Назначение |
|---|---|
| `HTTP_ADDR` | адрес HTTP-сервера |
| `LOG_LEVEL` | уровень логирования |
| `SHUTDOWN_TIMEOUT` | время graceful shutdown |
| `DATABASE_URL` | URL подключения к PostgreSQL |
| `DATABASE_MAX_CONNS` | максимум соединений в пуле |
| `DATABASE_MIN_CONNS` | минимум соединений в пуле |
| `DATABASE_MAX_CONN_LIFETIME` | время жизни соединения |
| `DATABASE_CONNECT_TIMEOUT` | таймаут подключения и стартового `Ping` |
| `DATABASE_QUERY_TIMEOUT` | таймаут SQL-запроса |

Все переменные обязательны. При отсутствующем или неверном значении сервис
завершается с ошибкой при старте.

## Локальный запуск

Команды выполняются из корня проекта в WSL:

```bash
tripgoctl cluster start
tripgoctl environment start
tripgoctl connect
make migrate
make run
```

Сервис доступен на `http://localhost:8080`.

```bash
curl -i http://localhost:8080/health
curl -i http://localhost:8080/ready
```

`/health` показывает, что процесс запущен. `/ready` выполняет `Ping`
PostgreSQL и при недоступной базе возвращает `503`.

## HTTP API

| Метод | Путь | Успешный ответ |
|---|---|---|
| `POST` | `/api/v1/trips` | `201 Created` и `Location` |
| `GET` | `/api/v1/trips/{tripId}` | `200 OK` |
| `POST` | `/api/v1/trips/{tripId}/finish` | `200 OK` |
| `GET` | `/health` | `200 OK` |
| `GET` | `/ready` | `200 OK` или `503 Service Unavailable` |

Пример создания поездки:

```bash
curl -i -X POST http://localhost:8080/api/v1/trips \
  -H 'Content-Type: application/json' \
  -d '{
    "user_id": "5cb72c04-7650-45c9-a79b-bcdba0631e0c",
    "driver_id": "8860b315-ec86-42eb-a17c-7c163d721ff5",
    "start_point": {"latitude": 59.9398, "longitude": 30.3146},
    "end_point": {"latitude": 59.9290, "longitude": 30.3626},
    "price": 1450
  }'
```

Ошибки возвращаются в `application/problem+json`:

| HTTP | `code` | Условие |
|---|---|---|
| `400` | `invalid_request` | неверный UUID или тело запроса |
| `404` | `trip_not_found` | поездка не найдена |
| `409` | `driver_busy` | у водителя есть активная поездка |
| `409` | `trip_completed` | повторное завершение поездки |
| `500` | `internal_error` | внутренняя ошибка |

## Команды проекта

```bash
make generate        # сгенерировать Go-код из OpenAPI
make test            # запустить go test -race ./...
make migrate         # применить все миграции
make migrate-down    # откатить одну миграцию
make migrate-status  # показать статус миграций
go build ./...       # проверить сборку
```

## Принятые решения

### Транзакции

`TransactionManager.Do` открывает транзакцию и сохраняет `pgx.Tx` в
`context.Context`. Репозиторий через `Executor` берёт транзакцию из контекста,
а вне `Do` использует пул.

- успешная функция внутри `Do` приводит к `COMMIT`;
- ошибка или паника приводит к `ROLLBACK`;
- вложенный `Do` переиспользует открытую транзакцию;
- поездка и начальная запись истории создаются атомарно;
- завершение поездки и новая запись истории также выполняются атомарно.

Используется уровень `Read Committed`. Конкурентное создание дополнительно
защищает уникальный индекс, а завершение — условный `UPDATE`, поэтому более
строгий уровень изоляции для этих операций не требуется.

### Конкурентность

Частичный уникальный индекс `trips_one_active_trip_per_driver_idx` запрещает
две активные поездки одного водителя. SQLSTATE `23505` для этого индекса
преобразуется в `409 driver_busy`.

Завершение обновляет только строку со `status = 'active'`. Из двух
конкурентных запросов один завершает поездку, второй получает
`409 trip_completed`.

## Docker: дополнительное задание

`deploy/Dockerfile` собирает статический бинарник на первой стадии. Итоговый
образ основан на `scratch`, содержит только `/trip-service` и запускает его с
UID/GID `65532:65532`. Исходники, Go toolchain, кеш сборки и локальные конфиги
в итоговый образ не попадают.

Размер собранного образа:

- `4.18 MB` — `CONTENT SIZE`;
- `15.5 MB` — `DISK USAGE` в локальном Docker;
- `11.3 MB` — несжатый слой с бинарником.

### Сборка и запуск

```bash
tripgoctl cluster start
tripgoctl environment start
make migrate

docker build \
  -f deploy/Dockerfile \
  -t localhost:5001/trip-service:lab1 \
  .

docker push localhost:5001/trip-service:lab1
```

Создать Secret с адресом PostgreSQL внутри Kubernetes:

```bash
kubectl create secret generic trip-service-config \
  --namespace tripgo-lab-01 \
  --from-literal=DATABASE_URL='postgres://tripgo:tripgo@postgres:5432/tripgo?sslmode=disable'
```

Запустить сервис:

```bash
kubectl apply -f deploy/trip-service.yaml
kubectl rollout status \
  deployment/trip-service \
  --namespace tripgo-lab-01 \
  --timeout=60s
```

Открыть локальный доступ:

```bash
kubectl port-forward \
  service/trip-service \
  18080:8080 \
  --namespace tripgo-lab-01
```

В другом терминале:

```bash
curl -i http://localhost:18080/ready
```

Ожидаемый ответ: `200 OK` и `{"status":"ok"}`.

### Остановка

```bash
kubectl delete -f deploy/trip-service.yaml
kubectl delete secret trip-service-config --namespace tripgo-lab-01
tripgoctl environment stop
```
