.PHONY: build build/docker build/docker/protoc build/docker/protos build/protos test help all

build/protos:
	@protoc -I . \
		--go_out=. \
		--go_opt=module=github.com/picatz/slogproto \
		slog.proto

build/docker/protoc:
	@docker build -t community-protoc -f Dockerfile.protoc .

build/docker/protos:
	@docker run --rm -v $(CURDIR):/workdir community-protoc:latest make build/protos

build/docker: build/docker/protoc build/docker/protos

build: build/docker

test:
	@go test -timeout 10m -v ./...

help:
	@echo "build:               build docker image"
	@echo "build/docker:        build docker image"
	@echo "build/docker/protoc: build docker image with protoc"
	@echo "build/docker/protos: build docker image with protoc and protos"
	@echo "build/protos:        build protos"
	@echo "test:                run tests"
	@echo "help:                print this help message"

all: build test