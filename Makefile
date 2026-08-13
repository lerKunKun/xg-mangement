.PHONY: infra-up infra-down migrate api worker web test verify

infra-up:
	docker compose up -d postgres redis rabbitmq minio minio-init

infra-down:
	docker compose down

migrate:
	docker compose run --rm migrate

api:
	cd backend && go run ./cmd/api

worker:
	cd backend && go run ./cmd/worker

web:
	npm --prefix apps/web run dev

test:
	cd backend && go test ./...
	npm --prefix apps/web test

verify:
	cd backend && go test ./... && go vet ./...
	npm --prefix apps/web test
	npm --prefix apps/web run lint
	npm --prefix apps/web run build
	docker compose config
