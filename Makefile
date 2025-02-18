.PHONY: build test clean proto docker docker-compose

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=goload
MASTER_BINARY=master
WORKER_BINARY=worker

# Build flags
LDFLAGS=-ldflags "-w -s"

# Docker parameters
DOCKER_IMAGE=goload
DOCKER_TAG=latest

all: clean deps proto build

build: 
	$(GOBUILD) $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/loadtest
	$(GOBUILD) $(LDFLAGS) -o bin/$(MASTER_BINARY) ./cmd/master
	$(GOBUILD) $(LDFLAGS) -o bin/$(WORKER_BINARY) ./cmd/worker

test:
	$(GOTEST) -v ./...

clean:
	$(GOCLEAN)
	rm -f bin/$(BINARY_NAME)
	rm -f bin/$(MASTER_BINARY)
	rm -f bin/$(WORKER_BINARY)
	rm -rf dist/

deps:
	$(GOMOD) download
	$(GOMOD) tidy

proto:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		api/proto/loadtest.proto

docker:
	docker build -t $(DOCKER_IMAGE):$(DOCKER_TAG) .

docker-compose:
	docker-compose up --build -d

docker-clean:
	docker-compose down -v
	docker rmi $(DOCKER_IMAGE):$(DOCKER_TAG)

run-master:
	./bin/$(MASTER_BINARY)

run-worker:
	./bin/$(WORKER_BINARY)

lint:
	golangci-lint run

format:
	gofmt -s -w .

# Development tools installation
install-tools:
	go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

# Cross compilation
build-linux:
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/linux/$(BINARY_NAME) ./cmd/loadtest
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/linux/$(MASTER_BINARY) ./cmd/master
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/linux/$(WORKER_BINARY) ./cmd/worker

build-darwin:
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/darwin/$(BINARY_NAME) ./cmd/loadtest
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/darwin/$(MASTER_BINARY) ./cmd/master
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/darwin/$(WORKER_BINARY) ./cmd/worker

build-windows:
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/windows/$(BINARY_NAME).exe ./cmd/loadtest
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/windows/$(MASTER_BINARY).exe ./cmd/master
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o dist/windows/$(WORKER_BINARY).exe ./cmd/worker

build-all: build-linux build-darwin build-windows