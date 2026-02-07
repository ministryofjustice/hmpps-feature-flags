SHELL = '/bin/bash'

-include .env
export

PROJECT_NAME = hmpps-feature-flags
SERVICE_NAME = flipt

COMPOSE_FILES = -f flipt/docker-compose.yml

export COMPOSE_PROJECT_NAME=${PROJECT_NAME}

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
	opa test flipt/policies/ -v

opa-lint: ## Runs the Regal linter on OPA policies.
	regal lint .

generate-acl: ## Generates ACL data from access.yml files.
	bash flipt/scripts/generate-acl-data.sh ./flags /tmp/acl-data.json
	@cat /tmp/acl-data.json

clean: ## Stops and removes all project containers and images.
	docker compose ${COMPOSE_FILES} down
	docker images -q --filter=reference="ghcr.io/ministryofjustice/*:local" | xargs -r docker rmi
	docker volume ls -qf "dangling=true" | xargs -r docker volume rm
