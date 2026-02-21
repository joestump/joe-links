.PHONY: build run migrate css clean tidy swagger dev dev-stop docker-build docker-up docker-down

BINARY := joe-links

build: css
	go build -o bin/$(BINARY) ./cmd/joe-links

run: css
	go run ./cmd/joe-links serve

migrate:
	go run ./cmd/joe-links migrate

css:
	npm run build

clean:
	rm -rf bin/ web/static/css/app.css node_modules/

tidy:
	go mod tidy

swagger:
	swag init -g internal/api/main_annotations.go -o docs/swagger --outputTypes json,yaml,go --parseDependency --parseInternal

dev:
	docker compose -f docker-compose.dev.yml up -d
	go run ./cmd/joe-links serve

dev-stop:
	docker compose -f docker-compose.dev.yml down

docker-build:
	docker build -t joe-links .

docker-up:
	docker compose up -d

docker-down:
	docker compose down

.DEFAULT_GOAL := build
