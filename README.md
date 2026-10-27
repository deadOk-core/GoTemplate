# TripGo

HTTP-сервис для создания, получения и завершения поездок. Данные хранятся в
PostgreSQL. HTTP-типы и серверные интерфейсы сгенерированы из OpenAPI-контракта.

В ЛР2 сервис разделён по слоям, добавлены тесты, линтер, JSON-логи, трейсы и
метрики. HTTP-контракт и миграции ЛР1 сохранены; новых ручек и миграций нет.

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
- WSL/Linux shell для команд `Makefile`;
- `golangci-lint` с поддержкой конфигурации v2; проверено на версии 2.15.0;
- C-компилятор и включённый CGO для тестов с `-race` (в Ubuntu — GCC).

`oapi-codegen`, `goose` и `mockgen` закреплены в секции `tool` файла `go.mod`
и запускаются через `go tool`; отдельная глобальная установка не нужна.

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
| `OTEL_SERVICE_NAME` | имя сервиса в телеметрии |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | базовый HTTP(S)-адрес OTel Collector |
| `OTEL_EXPORTER_OTLP_PROTOCOL` | `http/protobuf` |
| `OTEL_RESOURCE_ATTRIBUTES` | атрибуты ресурса, например namespace и окружение |
| `OTEL_TRACES_SAMPLER` | сэмплирование, например `parentbased_always_on` |

Все переменные обязательны. Приложение проверяет их наличие, адрес и протокол
OTLP, уровень логов, длительности и размеры пула. Ошибка проверки конфигурации
завершает запуск с ошибкой.

Стандартные настройки экспортёров и сэмплирования читает также OTel SDK.
Блоки `.env.example` для последующих лабораторных текущий сервис не использует.
`.env` и служебный каталог `.tripgo/` не коммитятся.

## Локальный запуск

Команды выполняются из корня проекта в WSL:

В `environment.toml` выбрана ЛР2: `lab = 2`, компоненты `postgres` и
`observability`. Docker daemon должен быть запущен.

```bash
tripgoctl cluster start
tripgoctl environment start
tripgoctl connect
make migrate
make run
```

Сервис доступен на `http://localhost:8080`.

Адрес сервиса задаётся `HTTP_ADDR`; адреса PostgreSQL, Grafana, Jaeger и
коллектора показывает `tripgoctl connect`. Они могут отличаться между
окружениями. Не нужно перезаписывать сгенерированный `.env` значениями из
`.env.example`.

Во втором терминале:

```bash
curl -i http://localhost:8080/health
curl -i http://localhost:8080/ready
```

`/health` показывает, что процесс запущен. `/ready` выполняет `Ping`
PostgreSQL и при недоступной базе возвращает `503`.

Для машины с небольшим объёмом памяти Go-команды можно запускать так:

```bash
GOMAXPROCS=2 GOFLAGS=-p=1 make migrate
GOMAXPROCS=2 GOFLAGS=-p=1 make run
```

Это ограничения параллельности конкретной команды. Конфигурация линтера из
курса, включая `run.concurrency: 8`, сохранена без изменений.

Если порт 8080 уже занят, сначала проверьте работающий процесс и `/ready`;
не запускайте второй экземпляр на том же порту. Остановка сервиса — `Ctrl+C`
в его терминале. Окружение останавливается отдельно:

```bash
tripgoctl environment stop
```

## Архитектура

```text
cmd/trip-service/main.go       конфигурация, запуск, сигналы и завершение
internal/
  config/                     чтение и валидация env
  container/                  создание и связывание конкретных зависимостей
  domain/trip/                сущность, валидация, переходы статусов, ошибки
  model/                      структуры для передачи данных между слоями
  usecase/trip/{create,get,finish}/
                              бизнес-сценарии, интерфейсы зависимостей
  http/trip/{create,get,finish}/
                              HTTP-разбор, вызов usecase, сборка ответа
  http/domainerror/           единый перевод доменных ошибок в Problem
  http/middleware/            Recover, Tracing, Logging, Metrics
  repository/trip/            SQL и преобразование ошибок PostgreSQL
  infra/postgres/             pgxpool и инструментирование запросов
  infra/postgres/transaction/ контекстный менеджер транзакций
  infra/telemetry/            slog, OTel SDK и метрика активных поездок
  generated/                  код, сгенерированный из OpenAPI
migrations/                   существующая схема PostgreSQL
```

Путь обработки запроса:

```text
HTTP handler → usecase → доменная модель
                  ↓
         интерфейс репозитория → SQL-репозиторий → PostgreSQL
         интерфейс TxManager  → менеджер транзакций
```

Стрелки к реализациям показывают вызовы во время работы. В исходном коде
usecase зависит от узких интерфейсов, а не от конкретного SQL-репозитория или
`pgx`. Домен не импортирует HTTP, `chi`, PostgreSQL и OpenTelemetry.
Хендлеры не выполняют SQL; репозиторий не формирует HTTP-ответы.

Интерфейсы объявлены неэкспортируемыми в `contract.go` пакета-потребителя:
хендлер задаёт нужный ему usecase, usecase — методы репозитория, менеджера
транзакций и счётчика. Потребитель не зависит от лишних методов реализации.
Моки генерируются рядом в `mocks_test.go`, без отдельного пакета `mocks`.

`internal/container` вручную создаёт SDK, пул, репозиторий, менеджер транзакций,
usecase, хендлеры и HTTP-сервер. `main` читает конфигурацию, вызывает контейнер
и управляет жизненным циклом; DI-фреймворк не используется. Глобальные
OTel-провайдеры настраиваются при инициализации инфраструктуры; бизнес-код
получает зависимости через конструкторы.

Доменные ошибки переводятся в HTTP только в `internal/http/domainerror`
через `errors.Is` и `errors.As`. Ответ `500` не содержит текста внутренней
ошибки; подробности остаются в логах.

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

Скопируйте `id` из ответа и выполните в том же терминале:

```bash
TRIP_ID='<id из ответа создания>'
curl -i "http://localhost:8080/api/v1/trips/$TRIP_ID"
curl -i -X POST "http://localhost:8080/api/v1/trips/$TRIP_ID/finish"
curl -i -X POST "http://localhost:8080/api/v1/trips/$TRIP_ID/finish"
```

Ожидаются `200`, `200`, затем `409 trip_completed`. Повторное создание с тем
же водителем до завершения его активной поездки возвращает `409 driver_busy`.

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
make generate        # OpenAPI и моки через go:generate
make lint            # golangci-lint run ./...
make test            # go test -race -count=2 -cover ./...
make test-integration # PostgreSQL, тег integration, -race -count=2
make migrate         # применить все миграции
make migrate-down    # откатить одну миграцию
make migrate-status  # показать статус миграций
go build ./...       # проверить сборку
```

Для форматирования: `golangci-lint fmt ./...` (`gofumpt` и `gci`).
Сгенерированные файлы коммитятся; вручную они не редактируются.

Unit-тестам БД не нужна. Домен проверяется table-driven тестами, usecase — с
`gomock` для репозитория и менеджера транзакций, хендлеры — через `httptest` с
моком usecase. Отдельно проверяются маппинг ошибок, конфигурация и телеметрия.

Для интеграционных тестов нужны работающее окружение ЛР2, `.env` и применённые
миграции. `make test-integration` загружает `.env`, передаёт `DATABASE_URL` как
`TEST_DATABASE_URL` и запускает тесты репозитория с тегом `integration`.
Тесты создают и удаляют свои данные; проверяются создание, чтение, завершение,
конфликт водителя и полный откат поездки вместе с историей при ошибке.

Для ограниченной параллельности:

```bash
GOMAXPROCS=2 GOFLAGS=-p=1 make generate
GOMAXPROCS=2 GOFLAGS=-p=1 make test
GOMAXPROCS=2 GOFLAGS=-p=1 make test-integration
GOMAXPROCS=2 go build -p 1 ./...
```

Покрытие каждого пакета домена и usecase должно быть не ниже 50%; `make test`
печатает покрытие. Линтер использует исходный конфиг курса без отключений.

В исходном `depguard` заданы правила для domain/usecase; отдельного правила
для HTTP-слоя и запрета локального `internal/generated` в нём нет. Эти
ограничения из матрицы зависимостей ТЗ дополнительно сверены по импортам.

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

Доменный `Trip.Finish` проверяет допустимость перехода статуса. Существующий
метод репозитория `Finish(ctx, id)` сохранён: условный SQL `UPDATE` устанавливает
`finished_at` временем PostgreSQL. Результат берётся из `RETURNING`.

Бизнес-логи и счётчики созданных/завершённых поездок записываются после
успешного завершения `Do`, включая `COMMIT`.

## Логи, трейсы и метрики

Порядок middleware:

```text
Recover → Tracing → Logging → Metrics → роутер
```

`Recover` перехватывает панику, пишет ошибку со стеком и формирует безопасный
`500 internal_error`. На запрос пишется один итоговый access-лог: `method`,
шаблон `route`, `status`, `duration_ms`, `trace_id`, `span_id`. Для `2xx` и
`4xx` используется `Info`, для `5xx` — `Error`. Тела запросов и ответов не
логируются. Бизнес-события `trip created` и `trip completed` пишутся отдельно.

`otelhttp` создаёт HTTP-спаны и поддерживает W3C `traceparent`. `otelpgx`
подключён на уровне пула; DB-спаны содержат `db.operation.name` и
`db.collection.name`. Ошибки спанов помечаются `codes.Error` и `error.type`.
В имени HTTP-спана и `http.route` используется шаблон, без UUID поездки.

Трейсы и метрики отправляются через OTLP HTTP/protobuf в OTel Collector.
Jaeger показывает отдельные запросы; Prometheus хранит метрики, Grafana
отображает их. Метрики HTTP/БД записываются адаптерами с лейблами курса;
встроенный сбор метрик библиотек отключён через noop-провайдеры для исключения
двойной записи. Недоступность коллектора не блокирует бизнес-запросы;
ошибки экспорта логируются.

| Имя инструмента OTel | Тип | Имя в Prometheus/Grafana |
|---|---|---|
| `http.server.request.duration` | histogram, секунды | `http_server_request_duration_seconds_{bucket,sum,count}` |
| `http.server.active_requests` | up-down counter | `http_server_active_requests` |
| `db.client.operation.duration` | histogram, секунды | `db_client_operation_duration_seconds_{bucket,sum,count}` |
| `tripgo.trips.created` | counter | `tripgo_trips_created_total` |
| `tripgo.trips.completed` | counter | `tripgo_trips_completed_total` |
| `tripgo.trips.active` | observable gauge | `tripgo_trips_active` |

Активные поездки читаются из БД при сборе gauge. Счётчики created/completed
относятся к текущему процессу и сбрасываются при его перезапуске.

При `SIGINT`/`SIGTERM` сначала завершается HTTP-сервер, затем SDK отправляет
оставшуюся телеметрию, после чего закрывается пул. Для финального сбора gauge
пул остаётся доступным до завершения SDK. Завершение ограничено
`SHUTDOWN_TIMEOUT`.

### Как посмотреть результат

1. Выполнить запросы создания, чтения и завершения поездки из раздела API.
2. В терминале сервиса найти `trip created` или `trip completed`, скопировать
   `trace_id` и открыть его в Jaeger через Lookup by Trace ID.
3. Раскрыть HTTP-спан и вложенные операции БД. При подготовке снимков получены
   trace ID `ec61c98d657ef4905484ef713698c85d` (создание) и
   `4287c77c3db671bcd62b1dc9c09d9512` (завершение).
4. В Grafana открыть Explore, выбрать Prometheus, режим Code и период,
   включающий время выполнения запросов. Адреса UI — из `tripgoctl connect`.
5. Выполнить запросы ниже. Метрики отправляются периодически; после первых
   HTTP-запросов следует подождать около минуты.

Текущие бизнес-метрики (`Type: Instant`, `Format: Table`):

```promql
{__name__=~"tripgo_trips_(created_total|completed_total|active)"}
```

Средняя длительность HTTP-запросов в секундах (`Type: Range`, `Format: Time series`):

```promql
sum by (http_request_method, http_route, http_response_status_code) (
  http_server_request_duration_seconds_sum
)
/
sum by (http_request_method, http_route, http_response_status_code) (
  http_server_request_duration_seconds_count
)
```

Средняя длительность DB-операций в секундах с теми же настройками:

```promql
sum by (db_operation_name, db_collection_name) (
  db_client_operation_duration_seconds_sum
)
/
sum by (db_operation_name, db_collection_name) (
  db_client_operation_duration_seconds_count
)
```

Оба выражения показывают накопленное среднее с начала работы процесса на
каждый момент графика. Это не перцентили и не среднее только за выбранное окно.
Grafana может предложить `rate()` для счётчиков; для показанного накопленного
среднего деление `sum / count` корректно.

Скриншоты трейсов и метрик прикладываются отдельным комментарием к PR.

## Docker: дополнительное задание ЛР1

Этот раздел фиксирует дополнительное задание и измерения версии ЛР1.
`deploy/trip-service.yaml` относится к `tripgo-lab-01` и не содержит обязательных
OTel-переменных текущего кода. Приведённые ниже команды Kubernetes относятся к
версии ЛР1; для текущей ЛР2 используйте локальный запуск выше. Размер образа
после добавления OTel заново не измерялся. Адаптация старого манифеста не входит
в обязательную часть ЛР2.

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
