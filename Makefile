# Makefile for SecretHold
#
# Usage:
#  make build       # Build the API server binary
#  make run         # Run the API server
#  make lint	    # Run linters on the code
#  make fmt    	    # Run formatters on the code
#  make pre-commit  # Run pre-commit hooks

BINARY_API    := bin/chirpy

build:
	go build -o $(BINARY_API) .

run: 
	./$(BINARY_API)

lint:
	gofumpt -w .
	golangci-lint run ./...

fmt:
	gofumpt -w .
