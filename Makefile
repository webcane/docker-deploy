.PHONY: build install test test-integration

build:
	mkdir -p bin
	go build -ldflags "-X main.version=dev" -o bin/docker-deploy ./cmd/docker-deploy/

install: build
	mkdir -p $(HOME)/.docker/cli-plugins
	install -m 755 bin/docker-deploy $(HOME)/.docker/cli-plugins/docker-deploy

test:
	go test ./...

test-integration:
	$(eval DOCKER_HOST ?= $(shell docker context inspect --format '{{(index .Endpoints "docker").Host}}' 2>/dev/null))
	DOCKER_HOST=$(DOCKER_HOST) TESTCONTAINERS_RYUK_DISABLED=true \
	  go test -tags integration -timeout 15m ./integration/...
