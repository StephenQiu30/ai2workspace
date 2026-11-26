.PHONY: generate check build run init-db

generate:
	go run github.com/swaggo/swag/cmd/swag@v1.16.6 init -d cmd/ai2workspace -g main.go --parseInternal --parseDependencyLevel 3 -o api --outputTypes go,json --quiet

check: generate
	test -z "$$(gofmt -l cmd internal tests)"
	go vet ./...
	go test -race ./...

build: generate
	go build ./...

run: generate
	go run ./cmd/ai2workspace

init-db:
	test -n "$$DATABASE_URL"
	psql "$$DATABASE_URL" --set=ON_ERROR_STOP=1 --file=schema.sql
