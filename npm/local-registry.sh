#!/usr/bin/env bash
# Publish the npm packages to a registry on this machine, and say where to look.
#
# The packages cannot be tried out the way a user meets them without a registry:
# installing a tarball by path skips the resolution that decides which of the
# four platform packages a machine downloads, which is the part most worth
# testing. Verdaccio is that registry, running on this machine, holding nothing
# anybody else can see.
#
#   npm/local-registry.sh          # build, publish, print the URL
#   npm/local-registry.sh stop     # stop it again
#
# Nothing here touches ~/.npmrc or the real registry. The credentials are a
# throwaway token in the state directory, passed with NPM_CONFIG_USERCONFIG so
# that no npm command in this script can reach npmjs.com by accident.
set -euo pipefail

cd "$(dirname "$0")/.."

PORT="${CS_LINT_REGISTRY_PORT:-4873}"
URL="http://localhost:$PORT"
STATE="npm/.local-registry"
CONFIG="$STATE/config.yaml"
NPMRC="$STATE/npmrc"
PIDFILE="$STATE/verdaccio.pid"

running() { curl -fsS -o /dev/null "$URL" 2>/dev/null; }

stop() {
  if [ -f "$PIDFILE" ]; then
    kill "$(cat "$PIDFILE")" 2>/dev/null || true
    rm -f "$PIDFILE"
    echo "stopped the registry on port $PORT"
  else
    echo "no registry started by this script is running"
  fi
}

if [ "${1:-}" = "stop" ]; then
  stop
  exit 0
fi

mkdir -p "$STATE"

# Verdaccio lives in the state directory rather than in this project's
# dependencies: it is a thing you run, not a thing cs-lint is built from, and
# this repository has no package.json of its own to record it in.
if [ ! -x "$STATE/node_modules/.bin/verdaccio" ]; then
  echo "==> installing verdaccio into $STATE (once)"
  npm install --silent --no-audit --no-fund --prefix "$STATE" verdaccio
fi

# Anonymous publish, because the only client is this script. A registry that
# holds four binaries and answers on localhost has nothing to authenticate.
cat > "$CONFIG" <<EOF
storage: ./storage
auth:
  htpasswd:
    file: ./htpasswd
uplinks: {}
packages:
  '**':
    access: \$anonymous
    publish: \$anonymous
    unpublish: \$anonymous
log: { type: stdout, format: pretty, level: warn }
EOF

# npm sends credentials even where none are wanted, so it is given some.
printf 'registry=%s/\n//localhost:%s/:_authToken=local-only\n' "$URL" "$PORT" > "$NPMRC"

if running; then
  echo "==> registry already up at $URL"
else
  echo "==> starting the registry on port $PORT"
  "$STATE/node_modules/.bin/verdaccio" --config "$CONFIG" --listen "$PORT" \
    > "$STATE/verdaccio.log" 2>&1 &
  echo $! > "$PIDFILE"
  for _ in $(seq 1 50); do
    running && break
    sleep 0.2
  done
  running || { echo "the registry did not come up; see $STATE/verdaccio.log" >&2; exit 1; }
fi

echo "==> building every platform"
goreleaser build --snapshot --clean --skip=before > "$STATE/build.log" 2>&1 ||
  { echo "the build failed; see $STATE/build.log" >&2; exit 1; }

echo "==> packaging"
node npm/build.mjs --dev

version="$(node -p "require('./npm/dist/cs-lint/package.json').version")"

# A version cannot be published twice, and a script meant to be run after every
# change would stop at the second run. Dropping the previous copy first is what
# makes this repeatable, and is only safe because the registry is this one.
echo "==> replacing $version, if it is already there"
for pkg in npm/dist/*/; do
  name="$(node -p "require('./$pkg/package.json').name")"
  NPM_CONFIG_USERCONFIG="$NPMRC" npm unpublish --force "$name@$version" >/dev/null 2>&1 || true
done

echo "==> publishing"
NPM_CONFIG_USERCONFIG="$NPMRC" CS_LINT_NPM_TAG=dev ./npm/publish.sh > "$STATE/publish.log" 2>&1 ||
  { echo "the publish failed; see $STATE/publish.log" >&2; exit 1; }

cat <<EOF

Published @codesweep-ai/cs-lint@$version to the registry on this machine.

  Browse   $URL/-/web/detail/@codesweep-ai/cs-lint
  Install  NPM_CONFIG_USERCONFIG=$PWD/$NPMRC npm install @codesweep-ai/cs-lint@dev
  Stop     npm/local-registry.sh stop
EOF
