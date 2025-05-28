prep:
	@go fmt ./...
	@go vet ./...
	@go get ./...
	@go mod tidy
	@go mod verify
	@go build -o /dev/null -v ./...

clean:
	@go clean -cache -modcache -testcache

update: prep
	go get -u ./...

install-lint:
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
	go install github.com/go-critic/go-critic/cmd/gocritic@latest
	go install github.com/securego/gosec/v2/cmd/gosec@latest

lint: prep install-lint
	golangci-lint run ./main.go
	gocritic check -enableAll ./main.go
	gosec -severity=high ./...

list:
	@go test -list . ./...
	@echo To run the selected test: make test n=TestMain

test: prep
	go test -v -cover --run $(n) ./...

test-all: prep
	go test -v -cover ./...

OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH := $(shell uname -m)
ifeq ($(ARCH),x86_64)
	ARCH := amd64
else ifeq ($(ARCH),aarch64)
	ARCH := arm64
endif

build:
	@CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -o lazyjournal

run: build
	@./lazyjournal
	@rm ./lazyjournal

BINPATH := $(HOME)/.local/bin

install: build
	@mkdir -p $(BINPATH)
	@mv ./lazyjournal $(BINPATH)/lazyjournal

VERSION := $(shell go run main.go -v)
OS_LIST := linux darwin openbsd freebsd windows
ARCH_LIST := amd64 arm64

build-all: prep
	@rm -rf bin
	@echo "Build version: $(VERSION)"
	@for os in $(OS_LIST); do \
		for arch in $(ARCH_LIST); do \
			ext=""; \
			if [ "$$os" = "windows" ]; then \
				ext=".exe"; \
			fi; \
			echo "Build lazyjournal-$(VERSION)-$$os-$$arch$$ext"; \
			CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -o bin/lazyjournal-$(VERSION)-$$os-$$arch$$ext || exit 1; \
		done; \
	done
	@ls -lh bin