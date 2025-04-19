VERSION := $(shell go run main.go -v)
BINPATH := $(HOME)/.local/bin

OS := $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH := $(shell uname -m)
ifeq ($(ARCH),x86_64)
	ARCH := amd64
else ifeq ($(ARCH),aarch64)
	ARCH := arm64
endif

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

build:
	CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH) go build -o lazyjournal

install: build
	@mkdir -p $(BINPATH)
	@mv ./lazyjournal $(BINPATH)/lazyjournal

build-all: prep
	@echo "Build version: $(VERSION)"
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/lazyjournal-$(VERSION)-linux-amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/lazyjournal-$(VERSION)-linux-arm64
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o bin/lazyjournal-$(VERSION)-darwin-amd64
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o bin/lazyjournal-$(VERSION)-darwin-arm64
	CGO_ENABLED=0 GOOS=openbsd GOARCH=amd64 go build -o bin/lazyjournal-$(VERSION)-openbsd-amd64
	CGO_ENABLED=0 GOOS=openbsd GOARCH=arm64 go build -o bin/lazyjournal-$(VERSION)-openbsd-arm64
	CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -o bin/lazyjournal-$(VERSION)-freebsd-amd64
	CGO_ENABLED=0 GOOS=freebsd GOARCH=arm64 go build -o bin/lazyjournal-$(VERSION)-freebsd-arm64
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/lazyjournal-$(VERSION)-windows-amd64.exe
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -o bin/lazyjournal-$(VERSION)-windows-arm64.exe
