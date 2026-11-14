.PHONY: generate run test lint test-integration migrate migrate-down migrate-status

generate:
	go tool oapi-codegen \
	-generate types,chi-server \
	-include-operation-ids createTrip,getTrip,finishTrip,health,ready \
	-package api \
	-o internal/generated/api.gen.go \
	contracts/openapi/trip-service.openapi.yaml
	go generate ./internal/...

run:
	@set -a; . ./.env; set +a; go run ./cmd/trip-service

test:
	go test -race -count=2 -cover ./...

migrate:
	@set -a; . ./.env; set +a; go tool goose -dir migrations postgres "$$DATABASE_URL" up

migrate-down:
	@set -a; . ./.env; set +a; go tool goose -dir migrations postgres "$$DATABASE_URL" down

migrate-status:
	@set -a; . ./.env; set +a; go tool goose -dir migrations postgres "$$DATABASE_URL" status

lint:
	golangci-lint run ./...

test-integration:
	@set -a; . ./.env; set +a; \
	TEST_DATABASE_URL="$$DATABASE_URL" go test -tags=integration -race -count=2 ./internal/repository/...