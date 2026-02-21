.PHONY: build run migrate css clean tidy swagger dev dev-stop docker-build docker-up docker-down ext-safari

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
	sudo go run ./cmd/joe-links serve

dev-stop:
	docker compose -f docker-compose.dev.yml down

docker-build:
	docker build -t joe-links .

docker-up:
	docker compose up -d

docker-down:
	docker compose down

# Convert the Web Extension to a Safari extension Xcode project.
# Requires Xcode command-line tools: xcode-select --install
# After running, open safari-extension/*.xcodeproj, build (Cmd+B), then
# enable the extension in Safari → Settings → Extensions.
ext-safari:
	xcrun safari-web-extension-converter extension/ \
		--app-name "joe-links" \
		--bundle-identifier "com.joestump.joe-links" \
		--swift \
		--no-prompt \
		--project-location safari-extension/

.DEFAULT_GOAL := build
