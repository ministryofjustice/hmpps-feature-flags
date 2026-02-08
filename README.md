# HMPPS Feature Flags

A self-hosted [Flipt](https://www.flipt.io/) (v2) instance for managing feature flags 
across HMPPS services.

| Environment | URL |
|---|---|
| Dev | https://feature-toggles-dev.hmpps.service.justice.gov.uk/ |
| Pre-Production | https://feature-toggles-preprod.hmpps.service.justice.gov.uk/ |
| Production | https://feature-toggles.hmpps.service.justice.gov.uk/ |

## Getting started (for teams)

### Prerequisites

- GitHub account, member of the `ministryofjustice` org
- Member of a GitHub team linked to the Flipt namespace you need

### Onboarding a new team

The quickest way to set up a new namespace is with the interactive wizard. Pull 
the project and run:

```sh
make new-namespace
```

This will prompt you for a namespace key, display name, description, and GitHub 
team slug(s), then scaffold all the required files across every environment 
and update CODEOWNERS for you.

If you'd prefer to do it manually, you need to create the following files 
for **each environment** (dev, preprod, prod):

```
flags/{env}/{namespace}/features.yml
flags/{env}/{namespace}/access.yml
```

**`features.yml`** defines the namespace and its flags:

```yaml
namespace:
    key: my-namespace
    name: My Namespace
    description: A short description of your service
```

**`access.yml`** grants write access to one or more GitHub teams:

```yaml
writers:
    - your-github-team-slug
    - another-team-slug
```

Once your files are in place, raise a PR to `main`. Once merged, the running 
instances will refresh within a minute with your new namespace.

### Adding new flags in DEV and PRE-PROD

In dev and pre-prod environments, you can create and manage flags 
directly through the Flipt UI:

1. Log in to the [Dev](https://feature-toggles-dev.hmpps.service.justice.gov.uk/) or [Pre-Prod](https://feature-toggles-preprod.hmpps.service.justice.gov.uk/) UI with your GitHub account
2. Navigate to your namespace
3. Create, update, or toggle flags as needed

Changes made through the UI are written back to the Git repository automatically. 
This makes dev and pre-prod a good place to iterate on flag configuration 
before promoting to production.

### Adding new flags in PROD

Production is **read-only** through the Flipt UI. All changes must 
go through a Git PR:

1. Create a new branch from `main`
2. Add or update your flags in `flags/prod/{namespace}/features.yml`
3. Run `make flags-lint` to validate your changes
4. Push the branch and raise a PR
5. Get approval from your team (CODEOWNERS enforces team-level review)
6. Merge to `main` — the change will deploy automatically through dev -> preprod -> prod

> [!TIP]
> You don't need to edit YAML by hand. The Flipt UI has a **Create branch** feature that lets you make flag changes visually on a new branch. Once you're happy with the changes, raise a PR from that branch for your team to review.

## Evaluating flags

The evaluation API is unauthenticated — services can evaluate flags without any tokens or credentials. Flipt is accessible only within the VPN/internal allowlist, and flag definitions are public in this repository.

Use the Flipt SDKs to evaluate flags from your services:

- [Client SDKs](https://github.com/flipt-io/flipt-client-sdks) — fetch flag state and evaluate locally in-memory (faster, recommended)
- [Server SDKs](https://github.com/flipt-io/flipt-server-sdks) — evaluate flags via API calls to the Flipt server

See the [Flipt docs](https://docs.flipt.io/introduction) for full API documentation.

## Authentication and authorization

**Authentication** for the Flipt management UI is via GitHub SSO. The evaluation API is excluded from authentication so services can read flags without credentials.

**Authorization** is enforced by OPA policies (`flipt/policies/namespace.rego`):

- **Team members** can create and update flags within namespaces they have access to (no delete)
- **Namespace access** is determined by the team mappings in each namespace's `access.yml`
- **Production** is read-only through the Flipt UI — changes must go through Git PRs

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
| `make new-namespace` | Interactive wizard to scaffold a new namespace |
| `make flags-validate` | Validate flag files using the Flipt CLI |
| `make flags-lint` | Check flag files match the canonical YAML format |
| `make flags-lint-fix` | Auto-format flag files to canonical YAML |
| `make opa-test` | Run OPA policy tests |
| `make opa-lint` | Lint Rego policies with Regal |
| `make generate-acl` | Generate ACL data from `access.yml` files |
| `make clean` | Remove all containers, images, and dangling volumes |

> [!TIP]
> You can run `make` commands sequentially like `make build up`

## Deployment
### Architecture

- **Git-backed storage** - flag definitions live in this repo under `flags/`, Flipt polls for changes
- **OPA authorization** - namespace-level access control via Rego policies
- **Dynamic ACL** - team access mappings are generated at runtime from `access.yml` files, no redeployment needed
- **Per-environment configs** - explicit Flipt config files baked into the Docker image (`flipt/config/`)

### Repository structure

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

Deployments are automated via GitHub Actions (`.github/workflows/pipeline.yml`). Pushing to `main` triggers a sequential rollout: dev -> preprod -> prod.

Each environment has its own Flipt config (`flipt/config/{dev,preprod,prod}.yml`) baked into the Docker image and selected via the `FLIPT_CONFIG_FILE` environment variable.
