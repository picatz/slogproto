.PHONY: generate test help all

generate:
	@buf generate

test:
	@go test -timeout 10m -v ./...

help:
	@echo "generate:            generate protos"
	@echo "test:                run tests"
	@echo "help:                print this help message"

all: generate test