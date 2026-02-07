#!/usr/bin/env sh
set -eu

log() {
  level="$1"; shift
  msg="$1"; shift
  if [ $# -gt 0 ]; then
    printf '%s\t%s\t%s\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$level" "$msg" "$1"
  else
    printf '%s\t%s\t%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$level" "$msg"
  fi
}

REPO_URL="${FLIPT_GIT_REPO_URL:-}"
REPO_BRANCH="${FLIPT_GIT_REPO_BRANCH:-main}"
REPO_PATH="${FLIPT_GIT_REPO_PATH:-/var/opt/flipt/repo}"
ACL_DATA_PATH="${FLIPT_AUTHORIZATION_LOCAL_DATA_PATH:-/var/opt/flipt/acl-data.json}"

# Clone repository if URL is configured and repo doesn't already exist
if [ -n "$REPO_URL" ] && [ ! -d "${REPO_PATH}/.git" ]; then
  log INFO "cloning repository" "{\"url\": \"$REPO_URL\", \"branch\": \"$REPO_BRANCH\"}"

  clone_url="$REPO_URL"
  if [ -n "${FLIPT_GIT_ACCESS_TOKEN:-}" ]; then
    clone_url=$(echo "$REPO_URL" | sed "s|https://|https://x-access-token:${FLIPT_GIT_ACCESS_TOKEN}@|")
  fi

  git clone --branch "$REPO_BRANCH" --single-branch --depth 1 "$clone_url" "$REPO_PATH"
  log INFO "repository cloned" "{\"path\": \"$REPO_PATH\"}"
fi

# Generate initial ACL data
if [ -d "${REPO_PATH}/flags" ]; then
  generate-acl-data "${REPO_PATH}/flags" "$ACL_DATA_PATH"
else
  echo '{"namespace_team_access":{}}' > "$ACL_DATA_PATH"
  log WARN "no flags directory found, created empty ACL data" "{\"path\": \"${REPO_PATH}/flags\"}"
fi

# Background loop to regenerate ACL data when repo updates
(
  while true; do
    sleep 30
    if [ -d "${REPO_PATH}/flags" ]; then
      generate-acl-data "${REPO_PATH}/flags" "$ACL_DATA_PATH" 2>/dev/null || true
    fi
  done
) &

# Start Flipt
CONFIG_FILE="${FLIPT_CONFIG_FILE:-/etc/flipt/config/default.yml}"
exec /flipt server --config "$CONFIG_FILE" "$@"
