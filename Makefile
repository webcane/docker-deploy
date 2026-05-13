.PHONY: build install test

build:
	mkdir -p bin
	go build -ldflags "-X main.version=dev" -o bin/docker-deploy ./cmd/docker-deploy/

install: build
	mkdir -p $(HOME)/.docker/cli-plugins
	install -m 755 bin/docker-deploy $(HOME)/.docker/cli-plugins/docker-deploy

test:
	go test ./...
