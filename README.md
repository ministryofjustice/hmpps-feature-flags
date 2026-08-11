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
team slug(s), then scaffold all the required files across every environment.

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
before promoting to production. If you'd rather raise a PR for these 
environments, the flag-approval bot approves it automatically - non-production 
changes don't need a review.

### Adding new flags in PROD

Production is **read-only** through the Flipt UI. All changes must 
go through a Git PR:

1. Create a new branch from `main`
2. Add or update your flags in `flags/prod/{namespace}/features.yml`
3. Run `make flags-lint` to validate your changes
4. Push the branch and raise a PR
5. Get approval from someone on your namespace's `writers` list - the 
   flag-approval bot requests and verifies this (skipped for 
   [self-service namespaces](#flag-review-policy))
6. Merge to `main` — the change will deploy automatically through dev -> preprod -> prod

> [!TIP]
> You don't need to edit YAML by hand. The Flipt UI has a **Create branch** feature that lets you make flag changes visually on a new branch. Once you're happy with the changes, raise a PR from that branch for your team to review.

### Flag review policy

Flag PRs are policed by the flag-approval bot - a required status check that 
reads each namespace's `access.yml` from `main` and comments on every PR with 
what it decided:

- **Non-production changes** are approved by the bot automatically - no 
  review needed
- **Production changes** block until someone on the namespace's `writers` 
  list approves - the bot requests the review and verifies the approver is 
  actually on one of those teams
- **Production changes in a self-service namespace** are approved by the bot 
  automatically. Opt in by adding `prodSelfService: true` to 
  `flags/prod/{namespace}/access.yml` - that change itself needs an admin 
  review, and it's the last one that does

A flag change raised by someone outside the namespace's `writers` needs a 
writer's approval, whatever the environment. Anything else - `access.yml` 
changes, files outside `flags/` - needs a human review through CODEOWNERS.

## Evaluating flags

The recommended approach is to use 
[`@flipt-io/flipt-client-js`](https://github.com/flipt-io/flipt-client-sdks/tree/main/flipt-client-js), 
which fetches flag state from Flipt and evaluates locally in-memory. This is faster 
than making API calls per evaluation and has no authentication requirement.

### Installation

```sh
npm install @flipt-io/flipt-client-js
```

### Setup

```typescript
import { FliptClient } from '@flipt-io/flipt-client-js';

const client = await FliptClient.init({
  url: 'https://feature-toggles.hmpps.service.justice.gov.uk',
  namespace: 'your-namespace',
  updateInterval: 120, // refresh flag state every 120 seconds
});
```

> [!NOTE]
> Due to Flipt being accessible only within the VPN/internal allowlist, and 
> flag states being public (as part of this repository), no `authentication` 
> option is needed — the evaluation API is open. Replace the URL with the 
> appropriate environment (see table above).

### Boolean flag evaluation

```typescript
const result = client.evaluateBoolean({
  flagKey: 'my-feature-flag',
  entityId: 'user-123',
  context: { region: 'north-west' },
});

if (result.enabled) {
  // feature is on for this user/context
}
```

### Variant flag evaluation

```typescript
const result = client.evaluateVariant({
  flagKey: 'template-version',
  entityId: 'user-123',
  context: { role: 'admin' },
});

console.log(result.variantKey); // e.g. "v2"
```

### Cleanup

In long-running Node.js services, call `close()` when shutting down to stop the 
background refresh timer:

```typescript
client.close();
```

For full SDK documentation, see the [Flipt client SDK docs](https://docs.flipt.io/integration/client) and [API reference](https://docs.flipt.io/introduction).

## Authentication and authorization

**Authentication** for the Flipt management UI is via GitHub SSO. The evaluation 
API is excluded from authentication so services can read flags without credentials.

**Authorization** is enforced by OPA policies (`flipt/policies/namespace.rego`):

- **Team members** can create and update flags within namespaces they have access to (no delete)
- **Namespace access** is determined by the team mappings in each namespace's `access.yml`
- **Production** is read-only through the Flipt UI — changes must go through Git PRs

## Local development

### Prerequisites

- Docker
- [OPA](https://www.openpolicyagent.org/) (`brew install opa`) - for running policy tests
- [Regal](https://docs.styra.com/regal) (`brew install styrainc/packages/regal`) - for linting policies
- Node.js 20+ - for running the smoke tests

### Running locally

```sh
make build   # Build the Docker image
make up      # Start Flipt at http://127.0.0.1:8080
```

GitHub OAuth and git pushes authenticate through the GitHub App, which needs a
`.env` file with:

```
FLIPT_GITHUB_CLIENT_ID=...
FLIPT_GITHUB_CLIENT_SECRET=...
FLIPT_GITHUB_APP_INSTALLATION_ID=...
FLIPT_GITHUB_APP_PRIVATE_KEY_B64=...
```

`FLIPT_GITHUB_APP_PRIVATE_KEY_B64` is the app's private key base64-encoded to a
single line (`base64 < the-app.private-key.pem | tr -d '\n'`) — `.env` values
can't span lines, so the container entrypoint decodes it back to the real key.
The client ID/secret are the GitHub App's — the app must list
`http://127.0.0.1:8080/auth/v1/method/github/callback` as a callback URL for
local login to work.

### Smoke tests

`make smoke-test` boots a disposable Flipt container and runs an end-to-end
suite (`tests/`) against it using the same `@flipt-io/flipt-client-js` client
that teams use: health, boolean/variant/segment evaluation, OPA namespace
authorization, and a flag mutation through the management API. The same suite
runs in the CI pipeline (which skips flag-only and CODEOWNERS-only changes).

The container uses `flipt/config/test.yml`, which deliberately has no
`storage.remote` and no credentials: flag changes made by the tests are commits
in a throwaway git repo inside the container and can never reach GitHub. The
flags under test are fixtures in `tests/fixtures/` — no real namespaces are
involved.

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
| `make smoke-test` | Run the smoke test suite against a disposable local Flipt instance |
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

Deployments are automated via GitHub Actions (`.github/workflows/pipeline.yml`). 
Pushing to `main` triggers a sequential rollout: dev -> preprod -> prod.

Each environment has its own Flipt config (`flipt/config/{dev,preprod,prod}.yml`) 
baked into the Docker image and selected via the `FLIPT_CONFIG_FILE` environment 
variable.
