all: build

.PHONY: build
build:
	go build

.PHONY: tidy
tidy:
	go mod tidy
