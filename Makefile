.PHONY: run chat test db test-pg lint build docker

## run: start the webhook server (reads .env)
run:
	set -a; [ -f .env ] && . ./.env; set +a; go run ./cmd/jodbanchee

## chat: talk to the bot in your terminal, no LINE account needed
chat:
	go run ./cmd/devchat

## test: run all tests with the race detector
test:
	go test -race ./...

## db: start a throwaway Postgres on :5432 for tests (needs Docker)
db:
	docker run -d --rm --name jodbanchee-pg -e POSTGRES_PASSWORD=postgres -p 5432:5432 postgres:17

## test-pg: run all tests, including the Postgres store's (run `make db` first)
test-pg:
	TEST_DATABASE_URL='postgres://postgres:postgres@localhost:5432/postgres?sslmode=disable' go test -race ./...

## lint: formatting and vet
lint:
	@test -z "$$(gofmt -l .)" || (echo "run gofmt on:"; gofmt -l .; exit 1)
	go vet ./...

## build: build the server binary into bin/
build:
	CGO_ENABLED=0 go build -trimpath -o bin/jodbanchee ./cmd/jodbanchee

## docker: build the container image
docker:
	docker build -t jodbanchee .