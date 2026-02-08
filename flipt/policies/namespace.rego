# regal ignore: directory-package-mismatch
package flipt.authz.v2

import rego.v1

default namespace_team_access := {}

namespace_team_access := data.namespace_team_access # regal ignore: unresolved-reference

auth_metadata := object.get(input.authentication, "metadata", {})

# regal ignore: line-length
teams_json := object.get(auth_metadata, "io.flipt.auth.github.teams", object.get(auth_metadata, "github_teams", "{\"ministryofjustice\":[]}"))
org_teams := json.unmarshal(teams_json)
teams := object.get(org_teams, "ministryofjustice", [])

has_correct_team if {
	some team in namespace_team_access[input.request.namespace]
	team in teams
}

# Prod guardrail:
# - The base production environment is read-only through Flipt.
# - Match both short key (`prod`) and display key (`Production`).
# - Users can branch the prod environment and make changes on the branch,
#   because branch environment keys are user-defined (e.g. "my-fix") and
#   won't match the base environment names below.
# - Changes go live when the branch is merged back via PR.
is_prod_environment if lower(input.request.environment) in {"prod", "production"}

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
	input.request.scope == "namespace"
	has_correct_team
	input.request.action != "delete"
	not is_prod_mutation
}

viewable_namespaces(_env) := ["*"] if {
	"hmpps-feature-flag-admins" in teams
}

viewable_namespaces(_env) := namespaces if {
	# regal ignore: external-reference
	not "hmpps-feature-flag-admins" in teams

	namespaces := [ns |
		# regal ignore: external-reference
		some ns, mapped_teams in namespace_team_access
		some t in mapped_teams

		# regal ignore: external-reference
		t in teams
	]
}
