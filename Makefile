IMAGE ?= ghcr.io/bhuv01/keda-queue-worker
TAG   ?= dev

.PHONY: help test build docker keda deploy-dev deploy-prod produce watch render clean

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-14s\033[0m %s\n",$$1,$$2}'

test: ## Run Go vet + tests
	cd worker && go vet ./... && go test -race -count=1 ./...

build: ## Build the Go binary locally
	cd worker && CGO_ENABLED=0 go build -o ../bin/app .

docker: ## Build the container image
	docker build -t $(IMAGE):$(TAG) worker

keda: ## Install KEDA core
	./scripts/00-install-keda.sh

deploy-dev: ## Apply the dev overlay (no ArgoCD)
	./scripts/01-deploy.sh dev

deploy-prod: ## Apply the prod overlay (no ArgoCD)
	./scripts/01-deploy.sh prod

produce: ## Enqueue messages to trigger scaling (COUNT=200 by default)
	./scripts/produce.sh

watch: ## Watch ScaledObject / HPA / pods
	./scripts/watch.sh

render: ## Render overlays locally (requires kustomize)
	@echo "### dev ###"  && kustomize build deploy/overlays/dev
	@echo "### prod ###" && kustomize build deploy/overlays/prod

clean: ## Remove local build artifacts
	rm -rf bin
