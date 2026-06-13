APP_NAME=ai-helloworld
HARNESS_DIR ?= .agent-harness
GCP_PROJECT ?= ai-helloworld-yan
REGION ?= asia-southeast1
REPOSITORY ?= backend-repo
SERVICE ?= summarizer
TAG ?= $(shell git rev-parse --short HEAD)
IMAGE ?= $(REGION)-docker.pkg.dev/$(GCP_PROJECT)/$(REPOSITORY)/$(SERVICE)

.PHONY: all lint test build run work dry-run harness-validate docker-build docker-push deploy gcp-init

all: lint test build

lint:
	golangci-lint run

test:
	go test ./... -v -coverprofile=coverage.out

build:
	go build -o bin/$(APP_NAME) ./cmd/app

run:
	set -a; \
	source .env; \
	set +a; \
	./bin/$(APP_NAME)

work:
	cd $(HARNESS_DIR) && $(MAKE) work

dry-run:
	cd $(HARNESS_DIR) && $(MAKE) dry-run

harness-validate:
	cd $(HARNESS_DIR) && $(MAKE) validate FEATURE=$(FEATURE)

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
