.PHONY: help deps tidy fmt vet test build run clean ci docker-build docker-run
.DEFAULT_GOAL := build

APP_NAME := melissa-bot
APP_VERSION := 0.1.0
APP_REPO := github.com/kaueabade
IMAGE ?= $(APP_REPO)/$(APP_NAME)
DOCKERFILE := build/package/Dockerfile
KUBE_FILE := deployments/podman/kube/melissa-bot.yml

include .env
help: ## Show available make targets
	@echo "Available commands:"
	@echo "  test        Run tests with race detector and coverage"
	@echo "  build       Build bot container image"
	@echo "  run         Run bot locally"
	@echo "  clean       Remove generated artifacts"

test: ## Run tests with race detector and coverage
	go test -race -cover ./...

build: ## Build container image
	@podman build -f $(DOCKERFILE) -t $(IMAGE):$(APP_VERSION) -t $(IMAGE):latest .

run: ## Run container image 
	@DEBUG=${DEBUG} DISCORD_BOT_TOKEN=${DISCORD_BOT_TOKEN} TZ=${TZ} WIPE_COMMANDS_ON_EXIT=${WIPE_COMMANDS_ON_EXIT} envsubst < $(KUBE_FILE) | podman kube play --replace -

stop: ## Stop running container
	@podman kube down $(KUBE_FILE) || true

clean: ## Remove generated artifacts
	@podman rmi -f $(IMAGE):$(APP_VERSION) $(IMAGE):latest || true
