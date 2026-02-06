# regal ignore: directory-package-mismatch
package flipt.authz.v2_test

import data.flipt.authz.v2 as flipt
import rego.v1

github_input(namespace, action, teams_json) := github_input_in_env(namespace, action, teams_json, "dev")

github_input_in_env(namespace, action, teams_json, environment) := {
	"authentication": {"metadata": {"io.flipt.auth.github.teams": teams_json}},
	"request": {"scope": "namespace", "environment": environment, "namespace": namespace, "action": action},
}

token_input(namespace, action, token_namespace) := token_input_in_env(namespace, action, token_namespace, "dev")

token_input_in_env(namespace, action, token_namespace, environment) := {
	"authentication": {"metadata": {"io.flipt.auth.token.namespace": token_namespace}},
	"request": {"scope": "namespace", "environment": environment, "namespace": namespace, "action": action},
}

test_team_namespace_allowed[action] if {
	some action in ["create", "update", "read"]

	# regal ignore: line-length
	flipt.allow with input as github_input("a-team", action, "{\"ministryofjustice\":[\"a-team\",\"another-team\"]}")
}

test_team_namespace_delete_not_allowed if {
	# regal ignore: line-length
	not flipt.allow with input as github_input("a-team", "delete", "{\"ministryofjustice\":[\"a-team\",\"another-team\"]}")
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
}

test_legacy_mapping_delete_not_allowed if {
	# regal ignore: line-length
	not flipt.allow with input as github_input("ProbationInCourt", "delete", "{\"ministryofjustice\":[\"hmpps-probation-in-court\"]}")
}

test_token_namespace_allowed[action] if {
	some action in ["create", "update", "delete", "read"]

	# regal ignore: line-length
	flipt.allow with input as token_input("a-team", action, "a-team")
}

test_token_namespace_not_allowed[action] if {
	some action in ["create", "update", "delete", "read"]

	# regal ignore: line-length
	not flipt.allow with input as token_input("a-team", action, "another-team")
}

test_admin_allowed[action] if {
	some action in ["create", "update", "delete", "read"]

	# regal ignore: line-length
	flipt.allow with input as github_input("random-namespace", action, "{\"ministryofjustice\":[\"hmpps-feature-flag-admins\"]}")
}

test_prod_team_namespace_read_allowed if {
	# regal ignore: line-length
	flipt.allow with input as github_input_in_env("a-team", "read", "{\"ministryofjustice\":[\"a-team\",\"another-team\"]}", "prod")
}

test_prod_team_namespace_update_not_allowed if {
	# regal ignore: line-length
	not flipt.allow with input as github_input_in_env("a-team", "update", "{\"ministryofjustice\":[\"a-team\",\"another-team\"]}", "prod")
}

test_prod_token_namespace_read_allowed if {
	flipt.allow with input as token_input_in_env("a-team", "read", "a-team", "prod")
}

test_prod_token_namespace_update_not_allowed if {
	not flipt.allow with input as token_input_in_env("a-team", "update", "a-team", "prod")
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
}

test_production_team_namespace_update_not_allowed if {
	# regal ignore: line-length
	not flipt.allow with input as github_input_in_env("a-team", "update", "{\"ministryofjustice\":[\"a-team\",\"another-team\"]}", "Production")
}

test_viewable_namespaces_admin if {
	# regal ignore: line-length
	flipt.viewable_namespaces("dev") == ["*"] with input as github_input("ignored", "read", "{\"ministryofjustice\":[\"hmpps-feature-flag-admins\"]}")
}

test_viewable_namespaces_team_includes_direct_and_legacy if {
	# regal ignore: line-length
	namespaces := flipt.viewable_namespaces("dev") with input as github_input("ignored", "read", "{\"ministryofjustice\":[\"a-team\",\"hmpps-probation-in-court\"]}")
	"a-team" in namespaces
	"ProbationInCourt" in namespaces
}
