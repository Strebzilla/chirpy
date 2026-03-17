# Makefile for Chirpy
#
# Usage:
#  make build       # Build the API server binary
#  make run         # Run the API server
#  make lint	    # Run linters on the code
#  make fmt    	    # Run formatters on the code

include .env
export

BINARY_API    := bin/chirpy

build:
	go build -o $(BINARY_API) .

run: build
	./$(BINARY_API)

lint: fmt
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

clean:
	rm -f $(BINARY_API)

sqlc:
	sqlc generate
