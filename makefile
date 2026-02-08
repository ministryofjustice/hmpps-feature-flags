SHELL = '/bin/bash'

-include .env
export

PROJECT_NAME = hmpps-feature-flags
SERVICE_NAME = flipt

COMPOSE_FILES = -f flipt/docker-compose.yml
GO_DIR = .go
GO_SCRIPTS = flipt/scripts

export COMPOSE_PROJECT_NAME=${PROJECT_NAME}

# Bootstrap the local Go build directory with a go.mod if it doesn't exist.
$(GO_DIR)/go.mod:
	@mkdir -p $(GO_DIR)
	@ln -sf $(CURDIR)/$(GO_SCRIPTS)/*.go $(GO_DIR)/
	@cd $(GO_DIR) && go mod init flipt-tools && go mod tidy

default: help

help: ## The help text you're reading.
	@grep --no-filename -E '^[0-9a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

build: ## Builds the Flipt Docker image.
	docker compose ${COMPOSE_FILES} build ${SERVICE_NAME}

up: ## Starts/restarts the local Flipt instance.
	docker compose ${COMPOSE_FILES} down ${SERVICE_NAME}
	docker compose ${COMPOSE_FILES} up ${SERVICE_NAME} --wait --no-recreate

down: ## Stops and removes all containers in the project.
	docker compose ${COMPOSE_FILES} down

opa-test: ## Runs the OPA policy test suite.
	@opa test flipt/policies/ -v

opa-lint: ## Runs the Regal linter on OPA policies.
	@regal lint .

flags-lint: $(GO_DIR)/go.mod ## Checks flag files match Flipt's canonical YAML format.
	@cd $(GO_DIR) && go run lint-flags.go ../flags

flags-lint-fix: $(GO_DIR)/go.mod ## Reformats flag files to Flipt's canonical YAML format.
	@cd $(GO_DIR) && go run lint-flags.go --fix ../flags

generate-acl: $(GO_DIR)/go.mod ## Generates ACL data from access.yml files.
	@cd $(GO_DIR) && go run generate-acl-data.go ../flags acl-data.json && cat acl-data.json

new-namespace: ## Interactive wizard to scaffold a new Flipt namespace.
	@bash scripts/new-namespace.sh

clean: ## Stops and removes all project containers and images.
	docker compose ${COMPOSE_FILES} down
	docker images -q --filter=reference="ghcr.io/ministryofjustice/*:local" | xargs -r docker rmi
	docker volume ls -qf "dangling=true" | xargs -r docker volume rm
