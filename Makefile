prep:
	@go fmt ./...
	@go vet ./...
	@go get ./...
	@go mod tidy
	@go mod verify
	@go build -o /dev/null -v ./...

clean:
	go clean -cache -modcache -testcache

update: prep
	go get -u ./...

lint-install:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/go-critic/go-critic/cmd/gocritic@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest

lint-check: lint-install
	golangci-lint run ./main.go
	gocritic check -enableAll ./main.go
	gosec -severity=high ./...

test-list:
	@go test -list . ./...
	@echo "\nTo run the selected test: \033[32mmake test n=TestMain\033[0m\n"

test: prep
	go test -v -cover --run $(n) ./...

test-all: prep
	go test -v -cover ./...

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

build-local:
	@CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -o lazyjournal

run: build-local
	@./lazyjournal
	@rm ./lazyjournal

BINPATH := $(HOME)/.local/bin

install-pre-built: build-local
	@mkdir -p $(BINPATH)
	@mv ./lazyjournal $(BINPATH)/lazyjournal

LAST_COMMIT_HASH := $(shell git ls-remote https://github.com/lifailon/lazyjournal HEAD | awk '{print $$1}')

install-last-commit:
	@GOBIN=$(BINPATH) go install github.com/Lifailon/lazyjournal@$(LAST_COMMIT_HASH)

uninstall:
	rm -f $(shell which lazyjournal)

SSH_OPTIONS := lifailon@192.168.3.101 -p 2121

copy:
	@tar czf - . | ssh $(SSH_OPTIONS) "mkdir -p git/lazyjournal && cd git/lazyjournal && tar xzf -"

run-remote: copy
	@ssh lifailon@192.168.3.101 -p 2121 -t "cd git/lazyjournal && /usr/local/go/bin/go run main.go"

test-remote: copy
	@ssh lifailon@192.168.3.101 -p 2121 "cd git/lazyjournal && /usr/local/go/bin/go test -v -cover ./..."