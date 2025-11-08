APP_NAME=ai-helloworld

.PHONY: all lint test build run docker

all: lint test build

lint:
	golangci-lint run

test:
	go test ./... -v -coverprofile=coverage.out

build:
	go build -o bin/$(APP_NAME) ./cmd/app

run:
	export LOG_LEVEL=debug
	./bin/$(APP_NAME)

docker:
	docker build -t $(APP_NAME):latest .
