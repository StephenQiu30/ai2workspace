SERVICE ?= identity
SERVICE_DIR := services/$(SERVICE)

.PHONY: generate check build run

generate:
	cd $(SERVICE_DIR) && GOWORK=off go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -g cmd/server/main.go --parseInternal -o api --outputTypes go,json --quiet

check: generate
	cd $(SERVICE_DIR) && test -z "$$(gofmt -l cmd internal tests)"
	cd $(SERVICE_DIR) && GOWORK=off go vet ./...
	cd $(SERVICE_DIR) && GOWORK=off go test -race -coverpkg=./internal/... ./tests/...

build: generate
	cd $(SERVICE_DIR) && GOWORK=off go build -o server ./cmd/server

run: generate
	cd $(SERVICE_DIR) && GOWORK=off go run ./cmd/server
