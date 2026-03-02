# regal ignore: directory-package-mismatch
package flipt.authz.v2

import rego.v1

default namespace_team_access := {}

namespace_team_access := data.namespace_team_access # regal ignore: unresolved-reference
default_environment := canonical_environment(data.authz_config.default_environment) if { # regal ignore: unresolved-reference
	data.authz_config.default_environment # regal ignore: unresolved-reference
}

default default_environment := ""

auth_metadata := object.get(input.authentication, "metadata", {})

# regal ignore: line-length
teams_json := object.get(auth_metadata, "io.flipt.auth.github.teams", object.get(auth_metadata, "github_teams", "{\"ministryofjustice\":[]}"))
org_teams := json.unmarshal(teams_json)
teams := object.get(org_teams, "ministryofjustice", [])

canonical_environment(environment) := "prod" if lower(environment) in {"prod", "production"}

canonical_environment(environment) := "preprod" if lower(environment) in {"preprod", "pre-prod", "pre-production"}

canonical_environment(environment) := lower(environment) if {
	not lower(environment) in {"prod", "production", "preprod", "pre-prod", "pre-production"}
}

# Branch guardrail:
# - Flipt sends user-defined branch names as request.environment for branch views.
# - ACL checks therefore fall back to the instance's configured default environment
#   when the request environment is not one of the generated ACL keys.
has_acl_environment(environment) if {
	object.get(namespace_team_access, canonical_environment(environment), null) != null
}

environment_namespace_team_access(environment) := object.get(
	namespace_team_access,
	canonical_environment(environment),
	{},
) if {
	has_acl_environment(environment)
}

environment_namespace_team_access(environment) := object.get(
	namespace_team_access,
	default_environment,
	{},
) if {
	not has_acl_environment(environment)
	default_environment != ""
}

environment_namespace_team_access(environment) := {} if {
	not has_acl_environment(environment)
	default_environment == ""
}

namespace_writer_teams := object.get(
	environment_namespace_team_access(input.request.environment),
	input.request.namespace,
	[],
)

has_correct_team if {
	some team in namespace_writer_teams
	team in teams
}

has_any_namespace_access(environment) if {
	some _, mapped_teams in environment_namespace_team_access(environment)
	some team in mapped_teams
	team in teams
}

# Prod guardrail:
# - The base production environment is read-only through Flipt.
# - Match both short key (`prod`) and display key (`Production`).
# - Users can branch the prod environment and make changes on the branch,
#   because branch environment keys are user-defined (e.g. "my-fix") and
#   won't match the base environment names below.
# - Changes go live when the branch is merged back via PR.
is_prod_environment if canonical_environment(input.request.environment) == "prod"

is_mutating_action if input.request.action in {"create", "update", "delete"}

is_prod_mutation if {
	is_prod_environment
	is_mutating_action
}

# Namespace guardrail:
# - Namespaces must be created/updated/deleted via Git PRs, not through the Flipt UI.
# - Namespace mutations use scope "environment" in Flipt v2.
is_namespace_mutation if {
	input.request.scope == "environment"
	is_mutating_action
}

default allow := false

# METADATA
# entrypoint: true
allow if {
	"hmpps-feature-flag-admins" in teams
	not is_prod_mutation
	not is_namespace_mutation
}

allow if {
	input.request.scope == "environment"
	input.request.action == "read"
	has_any_namespace_access(input.request.environment)
}

allow if {
	input.request.scope == "namespace"
	has_correct_team
	input.request.action != "delete"
	not is_prod_mutation
}

viewable_namespaces(env) := namespaces if {
	namespaces := [ns |
		# regal ignore: external-reference
		some ns, mapped_teams in environment_namespace_team_access(env)
		some t in mapped_teams

		# regal ignore: external-reference
		t in teams
	]
}
