.PHONY: up down run tidy build

# Start MongoDB + Redis in the background.
up:
	docker compose up -d

# Stop the infrastructure.
down:
	docker compose down

# Run the agent locally (loads .env). Requires `make up` first.
run:
	@set -a; . ./.env; set +a; go run .

# Sync dependencies.
tidy:
	go mod tidy

# Compile a binary.
build:
	go build -o bin/eywa-starter .
