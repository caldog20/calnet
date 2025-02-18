PROJECT_NAME = "calnet"
BIN_DIR ?= bin





all: manager node

control:
	go build -o $(BIN_DIR)/control/server cmd/controlserver/main.go

run-control: control
	./bin/control/server -config bin/control

node:

tidy:
	@go mod tidy

clean:
	rm -f $(BIN_DIR)/control/server

full-clean:
	rm -rf $(BIN_DIR)/*
	
.PHONY: tidy all control clean full-clean
