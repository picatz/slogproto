.PHONY: generate test help all

generate:
	@buf generate

test:
	@go test -timeout 10m -v ./...

install:
	@go install ./cmd/slp

help:
	@echo "install:             install slp"
	@echo "generate:            generate protos"
	@echo "test:                run tests"
	@echo "help:                print this help message"
	@echo "all:                 generate, test, install"

all: generate test install