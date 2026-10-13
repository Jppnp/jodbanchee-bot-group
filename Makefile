.PHONY: run chat test lint build docker

## run: start the webhook server (reads .env)
run:
	set -a; [ -f .env ] && . ./.env; set +a; go run ./cmd/jodbanchee

## chat: talk to the bot in your terminal, no LINE account needed
chat:
	go run ./cmd/devchat

## test: run all tests with the race detector
test:
	go test -race ./...

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
