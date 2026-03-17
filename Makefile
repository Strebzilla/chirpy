# Makefile for Chirpy

include .env
export

BINARY_API := bin/chirpy

build: clean
	go build -o $(BINARY_API) .

run: build
	./$(BINARY_API)

lint: fmt
	betteralign -apply ./...
	go mod tidy
	golangci-lint run ./...

fmt:	
	gofumpt -w .
	pg_format -i sql/**/*.sql

db-start:
	service postgresql start

db-up:
	goose up

db-down:
	goose down

db-reset:
	goose down-to 0

clean:
	rm -f $(BINARY_API)

sqlc:
	sqlc generate

test:
	go test ./...
