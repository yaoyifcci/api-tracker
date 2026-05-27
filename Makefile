.PHONY: build up down dev

build:
	cd backend && CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o api-tracker-server ./cmd/server

up:
	docker compose up --build -d

down:
	docker compose down

dev:
	@echo "Starting backend..."
	cd backend && go run ./cmd/server &
	@echo "Starting frontend..."
	cd frontend && npm run dev
