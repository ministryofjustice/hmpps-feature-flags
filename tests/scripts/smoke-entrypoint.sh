#!/usr/bin/env sh
set -eu

# Seed a throwaway git repo inside the container from the read-only fixture
# mount. Flipt only serves committed state, so the fixtures must be a real
# commit on main; with no storage.remote configured in test.yml, mutations
# commit to this repo only and can never reach GitHub.
REPO=/var/opt/flipt/test-repo
rm -rf "$REPO"
mkdir -p "$REPO"
cp -R /var/opt/flipt/fixtures/flags "$REPO/flags"
git -C "$REPO" init -q -b main
git -C "$REPO" add -A
git -C "$REPO" -c user.name="smoke-test" -c user.email="smoke-test@localhost" commit -q -m "Seed smoke test fixtures"

exec entrypoint
