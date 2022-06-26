all: build

.PHONY: build
build:
	go build

.PHONY: runpostgreexample
runpostgreexample:
	docker-compose -f cmd/examples/postgre/docker-compose.yml up --build

.PHONY: tidy
tidy:
	go mod tidy
