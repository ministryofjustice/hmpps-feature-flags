#!/usr/bin/env sh
set -eu

REPO_PATH="${FLIPT_GIT_REPO_PATH:-/var/opt/flipt/repo}"
ACL_DATA_PATH="${FLIPT_AUTHORIZATION_LOCAL_DATA_PATH:-/var/opt/flipt/acl-data.json}"
CONFIG_FILE="${FLIPT_CONFIG_FILE:-/etc/flipt/config/default.yml}"

# .env files can't hold multi-line values, so local runs supply the GitHub App
# private key base64-encoded and it's decoded here.
if [ -z "${FLIPT_GITHUB_APP_PRIVATE_KEY:-}" ] && [ -n "${FLIPT_GITHUB_APP_PRIVATE_KEY_B64:-}" ]; then
  FLIPT_GITHUB_APP_PRIVATE_KEY="$(printf '%s' "$FLIPT_GITHUB_APP_PRIVATE_KEY_B64" | base64 -d)"
  export FLIPT_GITHUB_APP_PRIVATE_KEY
fi

# Keep ACL data in sync as Flipt pulls repo updates
generate-acl-data --watch "${REPO_PATH}/flags" "$ACL_DATA_PATH" &

# Start Flipt
exec /flipt server --config "$CONFIG_FILE" "$@"
