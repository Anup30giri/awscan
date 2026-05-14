APP_NAME := awscan
BIN_DIR := bin

.PHONY: bootstrap build test fmt tidy

bootstrap:
	@echo "Install Go 1.22+ and ensure it is on PATH before building."
	@echo "Then run: go mod tidy"

build:
	go build -o $(BIN_DIR)/$(APP_NAME) .

test:
	go test ./...

fmt:
	go fmt ./...

tidy:
	go mod tidy
