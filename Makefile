.PHONY: generate run migrate migrate-down migrate-status

generate:
	go tool oapi-codegen \
	-generate types,chi-server \
	-include-operation-ids createTrip,getTrip,finishTrip,health,ready \
	-package api \
	-o internal/generated/api.gen.go \
	contracts/openapi/trip-service.openapi.yaml

run:
	@set -a; . ./.env; set +a; go run ./cmd/trip-service

migrate:
	@set -a; . ./.env; set +a; go tool goose -dir migrations postgres "$$DATABASE_URL" up

migrate-down:
	@set -a; . ./.env; set +a; go tool goose -dir migrations postgres "$$DATABASE_URL" down

migrate-status:
	@set -a; . ./.env; set +a; go tool goose -dir migrations postgres "$$DATABASE_URL" status