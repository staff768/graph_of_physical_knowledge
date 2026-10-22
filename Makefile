DATABASE_URL ?= postgres://physics:physics@localhost:5432/physics?sslmode=disable
TEST_DATABASE_URL ?= postgres://physics:physics@localhost:5432/physics_test?sslmode=disable
export DATABASE_URL

.PHONY: db seed check api web build test up down

db:            ## поднять PostgreSQL в docker
	docker compose up -d --wait postgres

seed:          ## загрузить content/*.yaml в БД
	go run ./cmd/seed

check:         ## проверить контент без БД
	go run ./cmd/seed -check

api:           ## запустить API на :8080
	go run ./cmd/api

web:           ## запустить фронтенд в dev-режиме на :5173
	cd web && npm install && npm run dev

build:         ## собрать фронтенд (его раздаёт API)
	cd web && npm ci && npm run build

test:          ## unit + интеграционные тесты (нужна запущенная БД)
	-docker compose exec -T postgres psql -U physics -c 'CREATE DATABASE physics_test' >/dev/null 2>&1
	TEST_DATABASE_URL=$(TEST_DATABASE_URL) go test ./...
	cd web && npx tsc -b

up:            ## поднять всё в docker: БД, seed, приложение на :8080
	docker compose --profile app up -d --build

down:
	docker compose --profile app down
