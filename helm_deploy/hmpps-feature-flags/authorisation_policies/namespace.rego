# regal ignore: directory-package-mismatch
package flipt.authz.v1

import rego.v1

allowed_teams := {
	"ProbationInCourt": ["hmpps-probation-in-court"],
	"ManageAWorkforce": ["manage-a-workforce"],
	"community-accomodation": ["hmpps-community-accommodation"],
	"probation-integration": ["probation-integration"],
	"manage-people-on-probation": ["hmpps-manage-people-on-probation"],
	"assess-risks-needs": ["hmpps-assess-risks-and-needs"],
	"consider-a-recall": ["making-recall-decision"],
	"hmpps-digital-prison-reporting-mi": ["hmpps-digital-prison-reporting"],
}

policy_input := input

org_teams := json.unmarshal(policy_input.authentication.metadata["io.flipt.auth.github.teams"])
teams := org_teams.ministryofjustice

in_explicitly_allowed_teams if {
	some team in allowed_teams[policy_input.request.namespace]
	team in teams
}

has_flipt_namespace_in_teams if policy_input.request.namespace in teams

has_correct_team if in_explicitly_allowed_teams

has_correct_team if has_flipt_namespace_in_teams

default allow := false

# allow if "hmpps-feature-flag-admins" in teams

allow if {
	policy_input.request.resource == "namespace"
	policy_input.request.action in ["read", "update", "create"]
	has_correct_team
}

allow if {
	policy_input.request.resource == "flag"
	has_correct_team
}

allow if {
	policy_input.request.resource == "segment"
	has_correct_team
}

allow if policy_input.request.resource == "authentication"
