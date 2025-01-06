PROJECT_NAME = "calnet"
BIN_DIR ?= bin





all: manager node

manager:
	go build -o $(BIN_DIR)/manager cmd/manager/main.go

run-manager: manager
	./bin/manager

node:

tidy:
	@go mod tidy

proto:
	@buf generate

buf-lint:
	@buf lint

deps:
	go install github.com/bufbuild/buf/cmd/buf@latest
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install connectrpc.com/connect/cmd/protoc-gen-connect-go@latest

clean:
	rm -rf $(BIN_DIR)/*
	rm -rf proto/gen

clean-db:
	@rm -f store.db
	
.PHONY: tidy proto buf-lint all manager client clean
