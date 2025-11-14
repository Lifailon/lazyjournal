prep:
	@go fmt ./...
	@go vet ./...
	@go get ./...
	@go mod tidy
	@go mod verify
	@go build -o /dev/null -v ./...

clean:
	go clean -cache -modcache -testcache

update:
	go get -u ./...

run: prep
	@go run main.go

# Linters

lint-install:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/go-critic/go-critic/cmd/gocritic@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest

lint:
	golangci-lint run -v ./main.go
	gocritic check -v -enableAll ./main.go
	gosec -severity=high ./...

lint-all:
	golangci-lint run -v ./main.go --no-config --enable-all

# Tests

test-list:
	@go test -list . ./...
	@echo "\nTo run the selected test: \033[32mmake test n=TestMain*\033[0m\n"

test:
	go test -v -cover --run $(n) ./...

test-all:
	go test -v -cover ./...

# Build

VERSION := $(shell go run main.go -v)

build-clear:
	@rm -rf bin

build-linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/lazyjournal-$(VERSION)-linux-amd64

build-linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/lazyjournal-$(VERSION)-linux-arm64

build-darwin-amd64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o bin/lazyjournal-$(VERSION)-darwin-amd64

build-darwin-arm64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o bin/lazyjournal-$(VERSION)-darwin-arm64

build-openbsd-amd64:
	CGO_ENABLED=0 GOOS=openbsd GOARCH=amd64 go build -o bin/lazyjournal-$(VERSION)-openbsd-amd64

build-openbsd-arm64:
	CGO_ENABLED=0 GOOS=openbsd GOARCH=arm64 go build -o bin/lazyjournal-$(VERSION)-openbsd-arm64

build-freebsd-amd64:
	CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -o bin/lazyjournal-$(VERSION)-freebsd-amd64

build-freebsd-arm64:
	CGO_ENABLED=0 GOOS=freebsd GOARCH=arm64 go build -o bin/lazyjournal-$(VERSION)-freebsd-arm64

build-windows-amd64:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/lazyjournal-$(VERSION)-windows-amd64.exe

build-windows-arm64:
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -o bin/lazyjournal-$(VERSION)-windows-arm64.exe

build-all-amd64: build-linux-amd64 build-darwin-amd64 build-openbsd-amd64 build-freebsd-amd64 build-windows-amd64

build-all-arm64: build-linux-arm64 build-darwin-arm64 build-openbsd-arm64 build-freebsd-arm64 build-windows-arm64

build-all: build-clear
	@make -j 10 build-all-amd64 build-all-arm64
	@ls -lh bin

OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH := $(shell uname -m)
ifeq ($(ARCH),x86_64)
	ARCH := amd64
else ifeq ($(ARCH),aarch64)
	ARCH := arm64
endif

build:
	@CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -o lazyjournal

run-bin: build
	@./lazyjournal
	@rm ./lazyjournal

# Install

BIN_PATH := $(HOME)/.local/bin

install-pre-built: build
	@mkdir -p $(BIN_PATH)
	@mv ./lazyjournal $(BIN_PATH)/lazyjournal

LAST_COMMIT_HASH := $(shell git ls-remote https://github.com/lifailon/lazyjournal HEAD | awk '{print $$1}')

install-last-commit:
	@GOBIN=$(BIN_PATH) go install github.com/Lifailon/lazyjournal@$(LAST_COMMIT_HASH)

uninstall:
	rm -f $(shell which lazyjournal)

# Remote

SSH_OPTIONS := lifailon@192.168.3.101 -p 2121
GO_PATH := /usr/local/go/bin/go

copy:
	@tar czf - . | ssh $(SSH_OPTIONS) "mkdir -p docker/lazyjournal && cd docker/lazyjournal && tar xzf -"

run-remote: copy
	@ssh $(SSH_OPTIONS) -t "cd docker/lazyjournal && $(GO_PATH) run main.go"

test-remote: copy
	@ssh $(SSH_OPTIONS) "cd docker/lazyjournal && $(GO_PATH) test -v -cover --run TestMockInterface ./..."

# Docker

docker-update:
	@docker compose pull

docker-run: docker-update
	@docker compose up -d
	@docker exec -it lazyjournal lazyjournal

docker-remove:
	@docker compose down
	@docker rmi lifailon/lazyjournal

K3S_ADDR := 192.168.3.101

k3s-config-prep:
	cp /etc/rancher/k3s/k3s.yaml $HOME/.kube/config
	sed -i "s/127.0.0.1/$(K3S_ADDR)/g" $HOME/.kube/config
	sed -i "s/# - $HOME\/.kube/- $HOME\/.kube/" docker-compose.yml