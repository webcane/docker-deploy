.PHONY: build install test test-ci lint fmt

build:
	mkdir -p bin
	go build -ldflags "-X main.version=dev \
		-X main.gitCommit=$(shell git rev-parse --short HEAD 2>/dev/null || echo unknown) \
		-X main.buildTime=$(shell date -u +%FT%TZ 2>/dev/null || echo unknown)" \
		-o bin/docker-deploy ./cmd/docker-deploy/

install: build
	mkdir -p $(HOME)/.docker/cli-plugins
	install -m 755 bin/docker-deploy $(HOME)/.docker/cli-plugins/docker-deploy

test:
	go test ./...

test-ci:
	@if [ -S /var/run/docker.sock ]; then \
		TESTCONTAINERS_RYUK_DISABLED=true \
		go test -v -tags integration -timeout 15m ./integration/... ; \
	else \
		TESTCONTAINERS_RYUK_DISABLED=true \
		DOCKER_HOST=unix://$(HOME)/.colima/default/docker.sock \
		go test -v -tags integration -timeout 15m ./integration/... ; \
	fi

lint:
	golangci-lint run ./...

fmt:
	find . -name '*.go' | xargs goimports -w -local github.com/webcane/docker-deploy
