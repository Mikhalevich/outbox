MKFILE_PATH := $(abspath $(lastword $(MAKEFILE_LIST)))
BUILD_PATH := $(dir $(MKFILE_PATH))
GOBIN ?= $(BUILD_PATH)tools/bin
LINTER_NAME := golangci-lint
LINTER_VERSION := v1.48.0

all: build

.PHONY: build
build:
	go build

.PHONY: test
test: lint
	go test ./...

.PHONY: runpostgreexample
runpostgreexample:
	docker-compose -f cmd/examples/postgre/docker-compose.yml up --build

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: install-linter
install-linter:
	if [ ! -f $(GOBIN)/$(LINTER_VERSION)/$(LINTER_NAME) ]; then \
		echo INSTALLING $(GOBIN)/$(LINTER_VERSION)/$(LINTER_NAME) $(LINTER_VERSION) ; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOBIN)/$(LINTER_VERSION) $(LINTER_VERSION) ; \
		echo DONE ; \
	fi

.PHONY: lint
lint: install-linter
	$(GOBIN)/$(LINTER_VERSION)/$(LINTER_NAME) run --config .golangci.yml
