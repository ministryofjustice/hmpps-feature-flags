# regal ignore: directory-package-mismatch
package flipt.authz.v1_test

import data.flipt.authz.v1 as flipt
import rego.v1

test_basics_allowed[resource_type][action] if {
	some resource_type in ["segment", "authentication", "flag"]
	some action in ["create", "update", "delete", "read"]

	# regal ignore: line-length
	flipt.allow with input as {"authentication": {"metadata": {"io.flipt.auth.github.organizations": "[\"ministryofjustice\"]", "io.flipt.auth.github.teams": "{\"ministryofjustice\":[\"a-team\",\"another-team\"]}"}}, "request": {"namespace": "a-team", "resource": resource_type, "subject": resource_type, "action": action, "status": "success"}}
}

test_basics_not_allowed[resource_type][action] if {
	some resource_type in ["segment", "flag", "namespace"]
	some action in ["create", "update", "delete", "read"]

	# regal ignore: line-length
	not flipt.allow with input as {"authentication": {"metadata": {"io.flipt.auth.github.organizations": "[\"ministryofjustice\"]", "io.flipt.auth.github.teams": "{\"ministryofjustice\":[\"a-team\",\"another-team\"]}"}}, "request": {"namespace": "random-namespace", "resource": resource_type, "subject": resource_type, "action": action, "status": "success"}}
}

test_basics_allowed_using_allowed_teams_map[resource_type][action] if {
	some resource_type in ["segment", "authentication", "flag"]
	some action in ["create", "update", "delete", "read"]

	# regal ignore: line-length
	flipt.allow with input as {"authentication": {"metadata": {"io.flipt.auth.github.organizations": "[\"ministryofjustice\"]", "io.flipt.auth.github.teams": "{\"ministryofjustice\":[\"hmpps-probation-in-court\"]}"}}, "request": {"namespace": "ProbationInCourt", "resource": resource_type, "subject": resource_type, "action": action, "status": "success"}}
}

test_namespaces_allowed_using_allowed_teams_map[action] if {
	some action in ["create", "update", "read"]

	# regal ignore: line-length
	flipt.allow with input as {"authentication": {"metadata": {"io.flipt.auth.github.organizations": "[\"ministryofjustice\"]", "io.flipt.auth.github.teams": "{\"ministryofjustice\":[\"hmpps-probation-in-court\"]}"}}, "request": {"namespace": "ProbationInCourt", "resource": "namespace", "subject": "namespace", "action": action, "status": "success"}}
}

test_basics_allowed_basic_token[resource_type][action] if {
	some resource_type in ["segment", "authentication", "flag"]
	some action in ["create", "update", "delete", "read"]

	# regal ignore: line-length
	flipt.allow with input as {"authentication": {"metadata": {"io.flipt.auth.token.namespace": "a-team"}}, "request": {"namespace": "a-team", "resource": resource_type, "subject": resource_type, "action": action, "status": "success"}}
}

test_namespaces_allowed_basic_token[action] if {
	some action in ["create", "update", "read"]

	# regal ignore: line-length
	flipt.allow with input as {"authentication": {"metadata": {"io.flipt.auth.token.namespace": "ProbationInCourt"}}, "request": {"namespace": "ProbationInCourt", "resource": "namespace", "subject": "namespace", "action": action, "status": "success"}}
}

test_namespaces_not_allowed_basic_token[action] if {
	action := "delete"

	# regal ignore: line-length
	not flipt.allow with input as {"authentication": {"metadata": {"io.flipt.auth.token.namespace": "ProbationInCourt"}}, "request": {"namespace": "ProbationInCourt", "resource": "namespace", "subject": "namespace", "action": action, "status": "success"}}
}

test_namespaces_allowed[action] if {
	some action in ["create", "update", "read"]

	# regal ignore: line-length
	flipt.allow with input as {"authentication": {"metadata": {"io.flipt.auth.github.organizations": "[\"ministryofjustice\"]", "io.flipt.auth.github.teams": "{\"ministryofjustice\":[\"a-team\",\"another-team\"]}"}}, "request": {"namespace": "a-team", "resource": "namespace", "subject": "namespace", "action": action, "status": "success"}}
}

test_namespaces_not_allowed[action] if {
	action := "delete"

	# regal ignore: line-length
	not flipt.allow with input as {"authentication": {"metadata": {"io.flipt.auth.github.organizations": "[\"ministryofjustice\"]", "io.flipt.auth.github.teams": "{\"ministryofjustice\":[\"a-team\",\"another-team\"]}"}}, "request": {"namespace": "a-team", "resource": "namespace", "subject": "namespace", "action": action, "status": "success"}}
}
