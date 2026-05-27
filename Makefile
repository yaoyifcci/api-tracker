.PHONY: build up up-local down dev

build:
	cd backend && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o api-tracker-server ./cmd/server

up:
	docker compose up --build -d

# Use this if your system proxy is on 127.0.0.1 (e.g. Clash/Surge on macOS with OrbStack)
up-local:
	docker compose -f docker-compose.yml -f docker-compose.local.yaml up --build -d

down:
	docker compose down

dev:
	@echo "Starting backend..."
	cd backend && go run ./cmd/server &
	@echo "Starting frontend..."
	cd frontend && npm run dev
