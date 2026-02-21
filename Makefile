.PHONY: build run migrate css clean tidy swagger

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

.DEFAULT_GOAL := build
