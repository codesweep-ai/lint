#!/usr/bin/env bash
# Serve a dev build of this repository's npm packages from this machine, so npm
# installs them the way a user meets them.
#
# Installing a tarball by path skips the resolution that decides which of the
# four platform packages a machine downloads, which is the part most worth
# testing. cs-npmrevs (https://github.com/codesweep-ai/npmrevs) makes every
# revision of an npm package installable without publishing it: it serves the
# packages built here, and passes every other package through from npmjs.com.
#
#   npm/local-registry.sh          # build, package, serve, and print how to install
#   npm/local-registry.sh stop     # stop the server again
#
# Nothing here touches ~/.npmrc or npmjs.com. The npmrc it writes lives in the
# state directory and is passed with NPM_CONFIG_USERCONFIG, and cs-npmrevs refuses
# every write, so nothing run through that npmrc can publish anywhere.
set -euo pipefail

cd "$(dirname "$0")/.."

PORT="${CS_LINT_REGISTRY_PORT:-4873}"
URL="http://127.0.0.1:$PORT"
STATE="npm/.local-registry"
DATA="$STATE/data"
NPMRC="$STATE/npmrc"
PIDFILE="$STATE/cs-npmrevs.pid"
# The command that runs cs-npmrevs: the version go.mod pins, unless told otherwise.
NPMREVS="${NPMREVS:-$(go tool -n cs-npmrevs)}"

answering() { curl -fsS -o /dev/null "$URL/-/ping" 2>/dev/null; }

# The pid of the cs-npmrevs answering on the port, when it serves this script's
# data directory. NPMREVS may be a launcher, such as `npx cs-npmrevs` or `go tool
# cs-npmrevs`, whose own pid is not the server's, so the server is asked.
# Empty when nothing answers, or something else does.
ours() {
  local status
  status="$(curl -fsS "$URL/-/npmrevs" 2>/dev/null | tr -d '\n ')" || return 0
  case "$status" in
  *"\"data\":[\"$(printf '%s' "$PWD/$DATA" | tr -d ' ')\"]"*)
    printf '%s' "$status" | sed -n 's/.*"pid":\([0-9]*\).*/\1/p'
    ;;
  esac
}

stop() {
  local pid launcher=""
  pid="$(ours)"
  if [ -n "$pid" ]; then
    kill "$pid" 2>/dev/null || true
  fi
  # The launcher too, when there was one, but only while that pid still runs
  # cs-npmrevs: a stale file must not name some other process that took the pid.
  if [ -f "$PIDFILE" ]; then
    launcher="$(cat "$PIDFILE")"
    if ps -o command= -p "$launcher" 2>/dev/null | grep -q cs-npmrevs; then
      kill "$launcher" 2>/dev/null || true
    else
      launcher=""
    fi
    rm -f "$PIDFILE"
  fi
  if [ -z "$pid" ] && [ -z "$launcher" ]; then
    echo "no registry started by this script is running"
    return
  fi
  for _ in $(seq 1 50); do
    answering || break
    sleep 0.1
  done
  echo "stopped the registry on port $PORT"
}

if [ "${1:-}" = "stop" ]; then
  stop
  exit 0
fi

mkdir -p "$DATA"

echo "==> building every platform"
goreleaser build --snapshot --clean --skip=before > "$STATE/build.log" 2>&1 ||
  { echo "the build failed; see $STATE/build.log" >&2; exit 1; }

echo "==> packaging"
node npm/build.mjs --dev

version="$(node -p "require('./npm/dist/lint/package.json').version")"
wrapper="$(node -p "require('./npm/dist/lint/package.json').name")"

# Packed into the data directory, which the server rereads as it changes. A
# version packed again replaces its file, so rebuilding a modified tree replaces
# what the last run packed.
echo "==> packing $wrapper@$version into $DATA"
for pkg in npm/dist/*/; do
  npm pack "$pkg" --pack-destination "$DATA" --silent >/dev/null
done

# The server this script started last is replaced, so the build of cs-npmrevs doing
# the serving is the current one. Anything else on the port is left alone.
stop >/dev/null
if answering; then
  echo "port $PORT is taken by a program this script did not start." >&2
  echo "Stop it, or set CS_LINT_REGISTRY_PORT to a free port." >&2
  exit 1
fi
echo "==> starting the registry on port $PORT"
# shellcheck disable=SC2086 # NPMREVS may be a command with arguments
$NPMREVS serve --data "$DATA" --listen "127.0.0.1:$PORT" > "$STATE/cs-npmrevs.log" 2>&1 &
echo $! > "$PIDFILE"
for _ in $(seq 1 50); do
  answering && break
  sleep 0.1
done
answering || { echo "the registry did not come up; see $STATE/cs-npmrevs.log" >&2; exit 1; }

# Every package goes to this registry, which serves the local ones and passes
# the rest through from npmjs.com.
printf 'registry=%s/\n' "$URL" > "$NPMRC"

cat <<MSG

Serving $wrapper@$version from this machine, and every other package from npmjs.com.

  Install  NPM_CONFIG_USERCONFIG=$PWD/$NPMRC npm install $wrapper@$version
  Status   curl $URL/-/npmrevs
  Stop     npm/local-registry.sh stop

A lockfile written through it names this machine for the packages built here.
\`go tool cs-npmrevs lockfile check\` finds those entries before one is committed.
MSG
