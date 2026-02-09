# regal ignore: directory-package-mismatch
package flipt.authz.v2_test

import data.flipt.authz.v2 as flipt
import rego.v1

github_input(namespace, action, teams_json) := github_input_in_env(namespace, action, teams_json, "dev")

github_input_in_env(namespace, action, teams_json, environment) := {
	"authentication": {"metadata": {"io.flipt.auth.github.teams": teams_json}},
	"request": {"scope": "namespace", "environment": environment, "namespace": namespace, "action": action},
}

env_scope_input(namespace, action, teams_json) := {
	"authentication": {"metadata": {"io.flipt.auth.github.teams": teams_json}},
	"request": {"scope": "environment", "environment": "dev", "namespace": namespace, "action": action},
}

test_team_namespace_allowed[action] if {
	some action in ["create", "update", "read"]

	# regal ignore: line-length
	flipt.allow with input as github_input("a-team", action, "{\"ministryofjustice\":[\"a-team\",\"another-team\"]}")
		with data.namespace_team_access as {"a-team": ["a-team"]} # regal ignore: unresolved-reference,line-length
}

test_team_namespace_delete_not_allowed if {
	# regal ignore: line-length
	not flipt.allow with input as github_input("a-team", "delete", "{\"ministryofjustice\":[\"a-team\",\"another-team\"]}")
		with data.namespace_team_access as {"a-team": ["a-team"]} # regal ignore: unresolved-reference,line-length
}

test_team_namespace_not_allowed[resource_action] if {
	some resource_action in ["create", "update", "delete", "read"]
	some action in ["create", "update", "delete", "read"]
	action == resource_action

	# regal ignore: line-length
	not flipt.allow with input as github_input("random-namespace", action, "{\"ministryofjustice\":[\"a-team\",\"another-team\"]}")
}

test_legacy_mapping_allowed[action] if {
	some action in ["create", "update", "read"]

	# regal ignore: line-length
	flipt.allow with input as github_input("ProbationInCourt", action, "{\"ministryofjustice\":[\"hmpps-probation-in-court\"]}")
		with data.namespace_team_access as {"ProbationInCourt": ["hmpps-probation-in-court"]} # regal ignore: unresolved-reference,line-length
}

test_legacy_mapping_delete_not_allowed if {
	# regal ignore: line-length
	not flipt.allow with input as github_input("ProbationInCourt", "delete", "{\"ministryofjustice\":[\"hmpps-probation-in-court\"]}")
		with data.namespace_team_access as {"ProbationInCourt": ["hmpps-probation-in-court"]} # regal ignore: unresolved-reference,line-length
}

test_explicit_mapping_hyphenated_namespace_allowed[action] if {
	some action in ["create", "update", "read"]

	# regal ignore: line-length
	flipt.allow with input as github_input("community-accommodation", action, "{\"ministryofjustice\":[\"hmpps-community-accommodation\"]}")
		with data.namespace_team_access as {"community-accommodation": ["hmpps-community-accommodation"]} # regal ignore: unresolved-reference,line-length
}

test_explicit_mapping_hyphenated_namespace_delete_not_allowed if {
	# regal ignore: line-length
	not flipt.allow with input as github_input("community-accommodation", "delete", "{\"ministryofjustice\":[\"hmpps-community-accommodation\"]}")
		with data.namespace_team_access as {"community-accommodation": ["hmpps-community-accommodation"]} # regal ignore: unresolved-reference,line-length
}

test_admin_allowed[action] if {
	some action in ["create", "update", "delete", "read"]

	# regal ignore: line-length
	flipt.allow with input as github_input("random-namespace", action, "{\"ministryofjustice\":[\"hmpps-feature-flag-admins\"]}")
}

test_prod_team_namespace_read_allowed if {
	# regal ignore: line-length
	flipt.allow with input as github_input_in_env("a-team", "read", "{\"ministryofjustice\":[\"a-team\",\"another-team\"]}", "prod")
		with data.namespace_team_access as {"a-team": ["a-team"]} # regal ignore: unresolved-reference,line-length
}

test_prod_team_namespace_update_not_allowed if {
	# regal ignore: line-length
	not flipt.allow with input as github_input_in_env("a-team", "update", "{\"ministryofjustice\":[\"a-team\",\"another-team\"]}", "prod")
}

test_prod_admin_read_allowed if {
	# regal ignore: line-length
	flipt.allow with input as github_input_in_env("random-namespace", "read", "{\"ministryofjustice\":[\"hmpps-feature-flag-admins\"]}", "prod")
}

test_prod_admin_update_not_allowed if {
	# regal ignore: line-length
	not flipt.allow with input as github_input_in_env("random-namespace", "update", "{\"ministryofjustice\":[\"hmpps-feature-flag-admins\"]}", "prod")
}

test_production_team_namespace_read_allowed if {
	# regal ignore: line-length
	flipt.allow with input as github_input_in_env("a-team", "read", "{\"ministryofjustice\":[\"a-team\",\"another-team\"]}", "Production")
		with data.namespace_team_access as {"a-team": ["a-team"]} # regal ignore: unresolved-reference,line-length
}

test_production_team_namespace_update_not_allowed if {
	# regal ignore: line-length
	not flipt.allow with input as github_input_in_env("a-team", "update", "{\"ministryofjustice\":[\"a-team\",\"another-team\"]}", "Production")
}

# Prod branch tests: branched environments get a user-defined key (e.g. "my-fix")
# which doesn't match "prod"/"production", so mutations are allowed on branches.
test_prod_branch_team_allowed[action] if {
	some action in ["create", "update", "read"]

	# regal ignore: line-length
	flipt.allow with input as github_input_in_env("a-team", action, "{\"ministryofjustice\":[\"a-team\",\"another-team\"]}", "my-prod-fix")
		with data.namespace_team_access as {"a-team": ["a-team"]} # regal ignore: unresolved-reference,line-length
}

test_prod_branch_admin_allowed[action] if {
	some action in ["create", "update", "delete", "read"]

	# regal ignore: line-length
	flipt.allow with input as github_input_in_env("random-namespace", action, "{\"ministryofjustice\":[\"hmpps-feature-flag-admins\"]}", "my-prod-fix")
}

test_namespace_mutation_blocked_for_admin[action] if {
	some action in ["create", "update", "delete"]

	# regal ignore: line-length
	not flipt.allow with input as env_scope_input("a-team", action, "{\"ministryofjustice\":[\"hmpps-feature-flag-admins\"]}")
}

test_namespace_read_allowed_for_admin if {
	# regal ignore: line-length
	flipt.allow with input as env_scope_input("a-team", "read", "{\"ministryofjustice\":[\"hmpps-feature-flag-admins\"]}")
}

test_namespace_mutation_blocked_for_team[action] if {
	some action in ["create", "update", "delete"]

	# regal ignore: line-length
	not flipt.allow with input as env_scope_input("a-team", action, "{\"ministryofjustice\":[\"a-team\",\"another-team\"]}")
}

test_viewable_namespaces_admin if {
	# regal ignore: line-length
	flipt.viewable_namespaces("dev") == ["*"] with input as github_input("ignored", "read", "{\"ministryofjustice\":[\"hmpps-feature-flag-admins\"]}")
}

test_viewable_namespaces_team_uses_access_mapping if {
	# regal ignore: line-length
	namespaces := flipt.viewable_namespaces("dev") with input as github_input("ignored", "read", "{\"ministryofjustice\":[\"a-team\",\"hmpps-probation-in-court\"]}")
		with data.namespace_team_access as {"a-team-ns": ["a-team"], "ProbationInCourt": ["hmpps-probation-in-court"]} # regal ignore: unresolved-reference,line-length
	"a-team-ns" in namespaces
	"ProbationInCourt" in namespaces
}
