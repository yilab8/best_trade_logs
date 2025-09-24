.PHONY: run run-seed test

run:
	go run ./cmd/server

run-seed:
	go run ./cmd/server --seed

test:
	go test ./...
