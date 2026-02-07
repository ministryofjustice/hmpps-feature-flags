# HMPPS Feature Flags

A self-hosted [Flipt](https://www.flipt.io/) (v2) instance for managing feature flags across HMPPS services.

| Environment | URL |
|---|---|
| Dev | https://feature-toggles-dev.hmpps.service.justice.gov.uk/ |
| Pre-Production | https://feature-toggles-preprod.hmpps.service.justice.gov.uk/ |
| Production | https://feature-toggles.hmpps.service.justice.gov.uk/ |

## Architecture

- **Git-backed storage** - flag definitions live in this repo under `flags/`, Flipt polls for changes
- **OPA authorization** - namespace-level access control via Rego policies
- **Dynamic ACL** - team access mappings are generated at runtime from `access.yml` files, no redeployment needed
- **Per-environment configs** - explicit Flipt config files baked into the Docker image (`flipt/config/`)

## Repository structure

```
flags/
  {dev,preprod,prod}/
    {namespace}/
      features.yml        # Flag and segment definitions
      access.yml          # GitHub team write access
flipt/
  config/                 # Flipt server configs (one per environment + local)
  policies/               # OPA Rego authorization policies
  scripts/                # Entrypoint and ACL generation scripts
  Dockerfile
  docker-compose.yml
helm_deploy/              # Kubernetes Helm charts and per-environment values
```

## Getting started (for teams)

### Prerequisites

- GitHub account, member of the `ministryofjustice` org
- Member of a GitHub team linked to the Flipt namespace you need

### Onboarding a new team

1. Create a namespace directory under each environment in `flags/`, e.g. `flags/dev/my-team/`, `flags/preprod/my-team/`, `flags/prod/my-team/`
2. Add a `features.yml` with your namespace metadata and flag definitions
3. If your namespace name doesn't exactly match your GitHub team slug, add an `access.yml`:
   ```yaml
   writers:
     - your-github-team-slug
   ```
4. Raise a PR - ACL data is regenerated automatically on deployment

### Creating API tokens

Generate a namespace-scoped API token in each environment for flag evaluation. Store it as a Kubernetes secret:

```sh
kubectl create secret generic flipt \
  --from-literal=URL=<flipt url for the environment> \
  --from-literal=API_KEY=<api token from flipt> \
  -n <your kubernetes namespace>
```

## Authentication and authorization

**Authentication** is via GitHub SSO (for the UI) or namespace-scoped API tokens (for services).

**Authorization** is enforced by OPA policies (`flipt/policies/namespace.rego`):

- **Team members** can create and update flags within namespaces they have access to (no delete)
- **API tokens** are scoped to a single namespace and can perform any action within it
- **Namespace access** is determined by either:
  - Direct match: your GitHub team slug matches the namespace name
  - Explicit mapping: your team is listed in the namespace's `access.yml`
- **Production** is read-only through the Flipt UI - changes must go through Git PRs to `flags/prod/`

## Evaluating flags

Use the Flipt SDKs:

- [Client SDKs](https://github.com/flipt-io/flipt-client-sdks) - client-side evaluation with caching
- [Server SDKs](https://github.com/flipt-io/flipt-server-sdks) - server-side evaluation

See the [Flipt docs](https://docs.flipt.io/introduction) for full API documentation.

## Local development

### Prerequisites

- Docker
- [OPA](https://www.openpolicyagent.org/) (`brew install opa`) - for running policy tests
- [Regal](https://docs.styra.com/regal) (`brew install styrainc/packages/regal`) - for linting policies

### Running locally

```sh
make build   # Build the Docker image
make up      # Start Flipt at http://127.0.0.1:8080
```

GitHub OAuth requires a `.env` file with:

```
FLIPT_LOCAL_GITHUB_CLIENT_ID=...
FLIPT_LOCAL_GITHUB_CLIENT_SECRET=...
```

### Available make targets

| Target | Description |
|---|---|
| `make build` | Build the Flipt Docker image |
| `make up` | Start/restart the local Flipt instance |
| `make down` | Stop and remove all containers |
| `make opa-test` | Run OPA policy tests |
| `make opa-lint` | Lint Rego policies with Regal |
| `make generate-acl` | Generate ACL data from `access.yml` files |
| `make clean` | Remove all containers, images, and dangling volumes |

## Deployment

Deployments are automated via GitHub Actions (`.github/workflows/pipeline.yml`). Pushing to `main` triggers a sequential rollout: dev -> preprod -> prod.

Each environment has its own Flipt config (`flipt/config/{dev,preprod,prod}.yml`) baked into the Docker image and selected via the `FLIPT_CONFIG_FILE` environment variable.
