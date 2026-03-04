prep:
	@go fmt ./...
	@go vet ./...
	@go get ./...
	@go mod tidy
	@go mod verify
	@go build -o /dev/null -v ./...

clear-cache:
	go clean -cache -modcache -testcache

update-dependencies:
	go get -u ./...

run: prep
	@go run main.go

# Remote

SSH_OPTIONS = lifailon@192.168.3.101 -p 2121
ROOT_PATH   = docker/lazyjournal
GO_PATH     = /usr/local/go/bin/go
DLV_PATH    = /home/lifailon/go/bin/dlv

copy:
	@tar czf - . | dd status=progress | ssh $(SSH_OPTIONS) "mkdir -p $(ROOT_PATH) && rm -rf $(ROOT_PATH)/* && cd $(ROOT_PATH) && tar xzf -"

run-remote: copy
	@ssh -t $(SSH_OPTIONS) "cd $(ROOT_PATH) && $(GO_PATH) run main.go"

build-debug:
	@ssh $(SSH_OPTIONS) "cd $(ROOT_PATH) && $(GO_PATH) build -gcflags='all=-N -l' -o bin/debug"

run-debug: copy build-debug
	@ssh -t $(SSH_OPTIONS) "killall dlv || true && cd $(ROOT_PATH) && $(DLV_PATH) exec bin/debug --headless --listen=:12345 --api-version=2 --log"

# Linters

lint-install:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.8.0

lint-run: lint-install
	golangci-lint run -v ./main.go

lint-fix: lint-install
	golangci-lint run --fix ./main.go

gocrit:
	go install github.com/go-critic/go-critic/cmd/gocritic@latest
	gocritic check -v -enableAll ./main.go

gosec:
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec -severity=high ./...

# Releaser

goreleaser-install:
	go install github.com/goreleaser/goreleaser/v2@latest

goreleaser-check: goreleaser-install
	goreleaser --clean --skip=publish --skip=validate

# Tests

test-list:
	@go test -list . ./...
	@echo "\nTo run the selected test: \033[32mmake test-run case=TestMain*\033[0m\n"

test-run:
	go test -v -cover --run $(case) ./...

test-all:
	go test -v -cover ./...

# Build

BIN_NAME = lazyjournal

OS   = $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH = $(shell uname -m)

ifeq ($(ARCH),x86_64)
	ARCH = amd64
else ifeq ($(ARCH),aarch64)
	ARCH = arm64
endif

go-install:
	@GO_VERSION_LATEST=$$(curl -sSL https://go.dev/VERSION?m=text | head -1) && \
	echo "Install $$GO_VERSION_LATEST for ${ARCH}" && \
	curl -L "https://go.dev/dl/$$GO_VERSION_LATEST.linux-${ARCH}.tar.gz" | sudo tar -xz -C /usr/local
	@echo 'export PATH="/usr/local/go/bin:$$PATH"' >> ~/.bashrc && . ~/.bashrc

go-check:
	@if ! command -v go >/dev/null 2>&1; then \
		$(MAKE) go-install; \
	fi

build: go-check
	@CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -o $(BIN_NAME)
	@echo "Build for $(OS)/$(ARCH) in $(shell pwd)/$(BIN_NAME)"

VERSION = $(shell go run main.go -v)

build-clear:
	@rm -rf bin

build-linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/$(BIN_NAME)-$(VERSION)-linux-amd64

build-linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/$(BIN_NAME)-$(VERSION)-linux-arm64

build-darwin-amd64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o bin/$(BIN_NAME)-$(VERSION)-darwin-amd64

build-darwin-arm64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o bin/$(BIN_NAME)-$(VERSION)-darwin-arm64

build-openbsd-amd64:
	CGO_ENABLED=0 GOOS=openbsd GOARCH=amd64 go build -o bin/$(BIN_NAME)-$(VERSION)-openbsd-amd64

build-openbsd-arm64:
	CGO_ENABLED=0 GOOS=openbsd GOARCH=arm64 go build -o bin/$(BIN_NAME)-$(VERSION)-openbsd-arm64

build-freebsd-amd64:
	CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -o bin/$(BIN_NAME)-$(VERSION)-freebsd-amd64

build-freebsd-arm64:
	CGO_ENABLED=0 GOOS=freebsd GOARCH=arm64 go build -o bin/$(BIN_NAME)-$(VERSION)-freebsd-arm64

build-windows-amd64:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/$(BIN_NAME)-$(VERSION)-windows-amd64.exe

build-windows-arm64:
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -o bin/$(BIN_NAME)-$(VERSION)-windows-arm64.exe

build-all-amd64: build-linux-amd64 build-darwin-amd64 build-openbsd-amd64 build-freebsd-amd64 build-windows-amd64

build-all-arm64: build-linux-arm64 build-darwin-arm64 build-openbsd-arm64 build-freebsd-arm64 build-windows-arm64

build-all: build-clear
	@make -j 10 build-all-amd64 build-all-arm64
	@ls -lh bin

# Install

BIN_PATH = $(HOME)/.local/bin

install: build
	install -D -m 755 $(BIN_NAME) $(BIN_PATH)/$(BIN_NAME)

uninstall:
	rm -f $(BIN_PATH)/$(BIN_NAME)