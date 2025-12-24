.PHONY: dev build css run-server setup

dev:
	make -j2 css run-server

run-server:
	@if command -v air > /dev/null; then \
		air; \
	else \
		echo "Air not found in PATH. Try 'make setup' or 'go run cmd/server/main.go'"; \
		go run cmd/server/main.go; \
	fi

build:
	go build -o tmp/main cmd/server/main.go

css:
	npx tailwindcss -i ./web/assets/input.css -o ./web/static/css/style.css --watch

setup:
	go install github.com/air-verse/air@latest
	npm init -y
	npm install -D tailwindcss
