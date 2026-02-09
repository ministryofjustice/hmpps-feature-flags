#!/usr/bin/env sh
set -eu

REPO_PATH="${FLIPT_GIT_REPO_PATH:-/var/opt/flipt/repo}"
ACL_DATA_PATH="${FLIPT_AUTHORIZATION_LOCAL_DATA_PATH:-/var/opt/flipt/acl-data.json}"
CONFIG_FILE="${FLIPT_CONFIG_FILE:-/etc/flipt/config/default.yml}"

# Keep ACL data in sync as Flipt pulls repo updates
generate-acl-data --watch "${REPO_PATH}/flags" "$ACL_DATA_PATH" &

# Start Flipt
exec /flipt server --config "$CONFIG_FILE" "$@"
