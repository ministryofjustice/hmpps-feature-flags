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

# regal ignore: line-length
token_namespace := object.get(auth_metadata, "io.flipt.auth.token.namespace", object.get(auth_metadata, "token_namespace", ""))

in_explicitly_allowed_teams if {
	some team in namespace_team_access[input.request.namespace]
	team in teams
}

has_flipt_namespace_in_teams if input.request.namespace in teams

has_correct_team if in_explicitly_allowed_teams

has_correct_team if has_flipt_namespace_in_teams

# Prod guardrail:
# - Production is read-only through Flipt regardless of auth method.
# - Match both short key (`prod`) and display key (`Production`).
# - Mutations must go through Git PRs to `flags/prod/**`.
is_prod_environment if lower(input.request.environment) in {"prod", "production"}

is_mutating_action if input.request.action in {"create", "update", "delete"}

is_prod_mutation if {
	is_prod_environment
	is_mutating_action
}

default allow := false

# METADATA
# entrypoint: true
allow if {
	"hmpps-feature-flag-admins" in teams
	not is_prod_mutation
}

allow if {
	input.request.scope == "namespace"
	token_namespace == input.request.namespace
	not is_prod_mutation
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

	# regal ignore: external-reference
	direct_teams := [t | some t in teams]
	legacy_namespaces := [ns |
		# regal ignore: external-reference
		some ns, mapped_teams in namespace_team_access
		some t in mapped_teams

		# regal ignore: external-reference
		t in teams
	]
	namespaces := array.concat(direct_teams, legacy_namespaces)
}
