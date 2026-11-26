SERVICE ?= identity
SERVICE_DIR := services/$(SERVICE)
SWAG := go run github.com/swaggo/swag/cmd/swag@v1.16.6 init --parseInternal -o api --outputTypes go,json --quiet
GOOSE := go run github.com/pressly/goose/v3/cmd/goose@v3.30.0

.PHONY: generate check build run studio-generate studio-check studio-build studio-run studio-migrate

generate:
	cd $(SERVICE_DIR) && GOWORK=off $(SWAG) -g cmd/server/main.go

check: generate
	cd $(SERVICE_DIR) && test -z "$$(gofmt -l cmd internal tests)"
	cd $(SERVICE_DIR) && GOWORK=off go vet ./...
	cd $(SERVICE_DIR) && GOWORK=off go test -race -coverpkg=./internal/... ./tests/...

build: generate
	cd $(SERVICE_DIR) && GOWORK=off go build -o server ./cmd/server

run: generate
	cd $(SERVICE_DIR) && GOWORK=off go run ./cmd/server

studio-generate:
	GOWORK=off $(SWAG) -d cmd/server,internal -g main.go

studio-check: studio-generate
	test -z "$$(gofmt -l cmd internal tests)"
	GOWORK=off go vet ./...
	GOWORK=off go test -race ./...

studio-build: studio-generate
	GOWORK=off go build -o ../build/studio ./cmd/server

studio-run: studio-generate
	GOWORK=off go run ./cmd/server

studio-migrate:
	test -n "$$STUDIO_DATABASE_URL"
	GOOSE_DRIVER=postgres GOOSE_DBSTRING="$$STUDIO_DATABASE_URL" GOWORK=off $(GOOSE) -env=none -dir migrations up
