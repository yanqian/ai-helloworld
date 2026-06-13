APP_NAME=ai-helloworld
HARNESS_DIR ?= .agent-harness
LOCAL_DB ?= data/ai-helloworld.db

# Legacy GCP deployment settings. Local development does not require these.
GCP_PROJECT ?= ai-helloworld-yan
REGION ?= asia-southeast1
REPOSITORY ?= backend-repo
SERVICE ?= summarizer
TAG ?= $(shell git rev-parse --short HEAD)
IMAGE ?= $(REGION)-docker.pkg.dev/$(GCP_PROJECT)/$(REPOSITORY)/$(SERVICE)

.PHONY: all init-local lint test build run local-run work dry-run harness-validate docker-build docker-push deploy legacy-gcp-deploy gcp-init legacy-gcp-init

all: init-local test build

init-local:
	mkdir -p $(dir $(LOCAL_DB))
	@echo "local SQLite database path: $(LOCAL_DB)"

lint:
	golangci-lint run

test:
	go test ./... -v -coverprofile=coverage.out

build:
	go build -o bin/$(APP_NAME) ./cmd/app

run: build init-local
	set -a; \
	if [ -f .env ]; then source .env; fi; \
	set +a; \
	./bin/$(APP_NAME)

local-run: run

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

deploy:
	@echo "GCP deployment is legacy and out of scope for the default local workflow."
	@echo "Use 'make legacy-gcp-deploy' only when intentionally exercising the old Cloud Run path."
	@exit 2

legacy-gcp-deploy: docker-push
	gcloud run deploy $(SERVICE) \
		--image $(IMAGE):$(TAG) \
		--project $(GCP_PROJECT) \
		--region $(REGION) \
		--platform managed \
		--allow-unauthenticated

gcp-init:
	@echo "GCP setup is legacy and not required for local SQLite development."
	@echo "Use 'make legacy-gcp-init' only when intentionally provisioning the old Cloud Run path."
	@exit 2

legacy-gcp-init:
	ALLOW_LEGACY_GCP_SETUP=true scripts/setup_gcp_project.sh ai-helloworld-yan ${BILLING_ACCOUNT_ID} asia-southeast1 backend-repo summarizer
