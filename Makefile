APP_NAME=ai-helloworld
GCP_PROJECT ?= ai-helloworld-yan
REGION ?= asia-southeast1
REPOSITORY ?= backend-repo
SERVICE ?= summarizer
TAG ?= $(shell git rev-parse --short HEAD)
IMAGE ?= $(REGION)-docker.pkg.dev/$(GCP_PROJECT)/$(REPOSITORY)/$(SERVICE)

.PHONY: all lint test build run docker-build docker-push deploy gcp-init

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

docker-build:
	docker build -t $(IMAGE):$(TAG) .

docker-push: docker-build
	docker push $(IMAGE):$(TAG)

deploy: docker-push
	gcloud run deploy $(SERVICE) \
		--image $(IMAGE):$(TAG) \
		--project $(GCP_PROJECT) \
		--region $(REGION) \
		--platform managed \
		--allow-unauthenticated

gcp-init:
	scripts/setup_gcp_project.sh ai-helloworld-yan ${BILLING_ACCOUNT_ID} asia-southeast1 backend-repo summarizer

