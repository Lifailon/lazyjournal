# Install Go

OS   = $(shell uname -s | tr '[:upper:]' '[:lower:]')
ARCH = $(shell uname -m)
ifeq ($(ARCH),x86_64)
	ARCH = amd64
else ifeq ($(ARCH),aarch64)
	ARCH = arm64
endif

GO_VERSION_LATEST = $(shell curl -sSLf https://go.dev/VERSION?m=text | head -n 1)
GO_VERSION ?= $(GO_VERSION_LATEST)

go-install:
	@echo "Install $(GO_VERSION) for $(ARCH)"
	@curl -sSLf "https://go.dev/dl/$(GO_VERSION).linux-${ARCH}.tar.gz" | sudo tar -xz -C /usr/local
	@echo 'export PATH="/usr/local/go/bin:$$PATH"' >> ~/.bashrc
	@/usr/local/go/bin/go version

# Updates

clean-cache:
	@go clean -cache -modcache -testcache

update-go:
	@go mod edit -go=$(GO_VERSION)
	@go mod tidyG

update-modules:
	@go get -u ./...
	@go mod tidy

# Local run

check:
	@go vet ./...
	@go build -v -o /dev/null ./...

run: check
	@go run main.go

# Remote run

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

# Tests

test-list:
	@go test -list . ./...

test-case-run:
	@sudo go test -v -cover --run $(case) ./...

test-all-run:
	@rm -rf test.json *.out
	@sudo go test -json -cover -coverprofile coverage.out ./... > test.json
	@go tool cover -func coverage.out | tail -n 1 | awk '{print $$NF}'

coverage-merge:
	@go install github.com/wadey/gocovmerge@latest
	@rm -rf merge-coverage.out
	@gocovmerge *.out > merge-coverage.out
	@go tool cover -func merge-coverage.out | tail -n 1 | awk '{print $$NF}'

coverage-markdown-report:
	@{ \
		echo "File Line Function Coverage"; \
		echo "- - - -"; \
		go tool cover -func=coverage.out | \
			sed -E 's/^.+\///' | \
			sed "s/:/ /g" | \
			head -n -1; \
	} | awk '{print "|" $$1 "|" $$2 "|" $$3 "|" $$4 "|"}' > COVERAGE_REPORT.md
	@cat COVERAGE_REPORT.md

tparse-install:
	@go install github.com/mfridman/tparse@latest

test-report: tparse-install
	@tparse -all -file test.json

test-markdown-report: tparse-install
	@tparse -all -format=markdown -file test.json >> TEST_REPORT.md
	@cat TEST_REPORT.md

# Linters

lint-install:
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest

lint-list: lint-install
	golangci-lint linters

lint-run: lint-install
	golangci-lint run -v ./...

lint-fix: lint-install
	golangci-lint run --fix ./...

gocrit:
	go install github.com/go-critic/go-critic/cmd/gocritic@latest
	gocritic check -v -enableAll ./...

gosec:
	go install github.com/securego/gosec/v2/cmd/gosec@latest
	gosec -severity=high ./...

# Release

goreleaser-install:
	go install github.com/goreleaser/goreleaser/v2@latest

goreleaser-check: goreleaser-install
	goreleaser --clean --skip=publish,validate,sign

# Build

APP_NAME = lazyjournal
VERSION = $(shell go run main.go -v)

build-clear:
	@rm -rf bin

build-linux-amd64:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/$(APP_NAME)-$(VERSION)-linux-amd64

build-linux-arm64:
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o bin/$(APP_NAME)-$(VERSION)-linux-arm64

build-darwin-amd64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o bin/$(APP_NAME)-$(VERSION)-darwin-amd64

build-darwin-arm64:
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o bin/$(APP_NAME)-$(VERSION)-darwin-arm64

build-openbsd-amd64:
	CGO_ENABLED=0 GOOS=openbsd GOARCH=amd64 go build -o bin/$(APP_NAME)-$(VERSION)-openbsd-amd64

build-openbsd-arm64:
	CGO_ENABLED=0 GOOS=openbsd GOARCH=arm64 go build -o bin/$(APP_NAME)-$(VERSION)-openbsd-arm64

build-freebsd-amd64:
	CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -o bin/$(APP_NAME)-$(VERSION)-freebsd-amd64

build-freebsd-arm64:
	CGO_ENABLED=0 GOOS=freebsd GOARCH=arm64 go build -o bin/$(APP_NAME)-$(VERSION)-freebsd-arm64

build-windows-amd64:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -o bin/$(APP_NAME)-$(VERSION)-windows-amd64.exe

build-windows-arm64:
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -o bin/$(APP_NAME)-$(VERSION)-windows-arm64.exe

build-all-amd64: build-linux-amd64 build-darwin-amd64 build-openbsd-amd64 build-freebsd-amd64 build-windows-amd64

build-all-arm64: build-linux-arm64 build-darwin-arm64 build-openbsd-arm64 build-freebsd-arm64 build-windows-arm64

build-all: build-clear
	@make -j 10 build-all-amd64 build-all-arm64
	@ls -lh bin

# Install

INSTALL_PATH = $(HOME)/.local/bin

build:
	@CGO_ENABLED=0 go build -o $(APP_NAME)

install: build
	@mkdir -p $(INSTALL_PATH)
	@install -m 755 $(APP_NAME) $(INSTALL_PATH)/$(APP_NAME)

uninstall:
	rm -f $(INSTALL_PATH)/$(APP_NAME)

# Git

git-log:
	git --no-pager log --all --reverse --date=format:'%Y-%m-%d %H:%M:%S' \
		--pretty=format:"%ad%Creset %C(yellow)[%C(green)%an%Creset -> %C(cyan)https://github.com/Lifailon/lazyjournal/commit/%h%C(yellow)]%C(auto)%d%Creset %s"

git-changelog:
	git --no-pager log "$$(git describe --tags --abbrev=0)..HEAD" --no-merges --pretty=format:"- [%h](https://github.com/Lifailon/lazyjournal/commit/%h): %s"