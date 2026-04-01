# Makefile for Docker operations

LOCAL_IMAGE_TAG ?= 11.6.0-oidc
LOCAL_IMAGE_NAME ?= mattermost-server:$(LOCAL_IMAGE_TAG)

.PHONY: docker-build-local
docker-build-local: ## Build a local production-style Docker image from source.
	$(MAKE) build-linux-amd64 package-prep BUILD_ENTERPRISE=true BUILD_NUMBER=$(BUILD_NUMBER) BUILD_DATE="$(BUILD_DATE)" BUILD_HASH=$(BUILD_HASH)
	$(MAKE) package-general DIST_PATH_GENERIC=dist/mattermost CURRENT_PACKAGE_ARCH=linux_amd64 MM_BIN_NAME=mattermost MMCTL_BIN_NAME=mmctl BUILD_ENTERPRISE=true BUILD_NUMBER=$(BUILD_NUMBER) BUILD_DATE="$(BUILD_DATE)" BUILD_HASH=$(BUILD_HASH)
	docker build --platform linux/amd64 -t $(LOCAL_IMAGE_NAME) -f build/Dockerfile.local .

.PHONY: docker-run-local
docker-run-local: ## Run the locally built Docker image with dependencies.
	@export CURRENT_UID=$$(id -u):$$(id -g) && \
	unset DOCKER_DEFAULT_PLATFORM && \
	docker compose -f docker-compose.test.yaml up -d --remove-orphans
	@echo "Mattermost will be available at http://localhost:8065"

.PHONY: docker-stop-local
docker-stop-local: ## Stop the local Docker test environment.
	docker compose -f docker-compose.test.yaml down

.PHONY: docker-logs-local
docker-logs-local: ## Follow logs from the local Docker test environment.
	docker logs -f mattermost-server-test
