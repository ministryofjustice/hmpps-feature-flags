# regal ignore: directory-package-mismatch
package flipt.authz.v2

import rego.v1

# Local auth metadata shim:
# - Production metadata keys (io.flipt.*) are emitted by Flipt auth integrations.
# - Local static token metadata uses simple keys to avoid dot-key parsing issues.
auth_metadata := object.get(input.authentication, "metadata", {})

teams_json := object.get(auth_metadata, "io.flipt.auth.github.teams", object.get(auth_metadata, "github_teams", "{\"ministryofjustice\":[]}"))
org_teams := json.unmarshal(teams_json)
teams := object.get(org_teams, "ministryofjustice", [])

token_namespace := object.get(auth_metadata, "io.flipt.auth.token.namespace", object.get(auth_metadata, "token_namespace", ""))

allowed_teams := {
	"ProbationInCourt": ["hmpps-probation-in-court"],
	"ManageAWorkforce": ["manage-a-workforce"],
	"community-accomodation": ["hmpps-community-accommodation"],
	"probation-integration": ["probation-integration"],
	"manage-people-on-probation": ["hmpps-manage-people-on-probation"],
	"assess-risks-needs": ["hmpps-assess-risks-and-needs"],
	"consider-a-recall": ["making-recall-decision"],
}

in_explicitly_allowed_teams if {
	some team in allowed_teams[input.request.namespace]
	team in teams
}

has_flipt_namespace_in_teams if input.request.namespace in teams

has_correct_team if in_explicitly_allowed_teams
has_correct_team if has_flipt_namespace_in_teams

default allow := false

is_prod_environment if lower(input.request.environment) in {"prod", "production"}

is_mutating_action if input.request.action in {"create", "update", "delete"}

is_prod_mutation if {
	is_prod_environment
	is_mutating_action
}

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
	not "hmpps-feature-flag-admins" in teams
	direct_teams := [t | some t in teams]
	legacy_namespaces := [ns |
		some ns, mapped_teams in allowed_teams
		some t in mapped_teams
		t in teams
	]
	namespaces := array.concat(direct_teams, legacy_namespaces)
}
