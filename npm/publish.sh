#!/usr/bin/env bash
# Publish what npm/build.mjs wrote.
#
# The four platform packages go first and the wrapper goes last, which is the
# only ordering that is safe. The wrapper is the name people install, and it
# depends on packages that must already exist when it does: publish it first
# and every install between the two commands resolves a wrapper whose binary is
# not on the registry yet.
#
#   node npm/build.mjs        # after a goreleaser release
#   npm/publish.sh --dry-run  # what would go, and what is in each package
#   npm/publish.sh
#
# Publishing is authenticated by whatever the caller already has. In CI that is
# a trusted publisher, so no token is stored: the workflow needs `id-token:
# write`, and npm mints provenance from the same OIDC identity that signs the
# release checksums with cosign.
set -euo pipefail

cd "$(dirname "$0")/.."
dist="npm/dist"

[ -d "$dist" ] || {
  echo "npm/publish.sh: no $dist. Run \`node npm/build.mjs\` first." >&2
  exit 1
}

dry=""
if [ "${1:-}" = "--dry-run" ]; then
  dry="--dry-run"
  shift
fi

# The wrapper is named explicitly rather than found last by a glob, so a
# renamed directory cannot reorder the publish into the unsafe order.
wrapper="$dist/cs-lint"
platforms=("$dist"/cs-lint-*)

[ -d "$wrapper" ] || {
  echo "npm/publish.sh: $wrapper is missing; the build did not finish." >&2
  exit 1
}
[ "${#platforms[@]}" -eq 4 ] || {
  echo "npm/publish.sh: found ${#platforms[@]} platform packages, expected 4." >&2
  exit 1
}

version="$(node -p "require('./$wrapper/package.json').version")"

# A prerelease must not become the version a bare `npm install` picks, and npm
# refuses to guess: without a tag it stops rather than quietly moving `latest`.
# Naming the tag here keeps a release candidate installable, by people who ask
# for it, without putting it in front of everyone else.
# CS_LINT_NPM_TAG names the channel. A dev build off any commit belongs on one
# of its own rather than on `next`, which a release candidate will want.
tag=()
case "$version" in
*-*) tag=(--tag "${CS_LINT_NPM_TAG:-next}") ;;
esac

# Provenance is minted from the CI identity that runs the publish, so it can
# only be asked for where there is one. A bootstrap publish from a laptop has
# no OIDC token, and asking there fails the publish outright.
provenance=()
if [ -n "${GITHUB_ACTIONS:-}" ]; then
  provenance=(--provenance)
else
  echo "note: publishing without provenance, which needs CI. ${dry:+(dry run) }" >&2
fi

for pkg in "${platforms[@]}" "$wrapper"; do
  echo "==> $(node -p "require('./$pkg/package.json').name")"
  npm publish ${dry:+$dry} "${tag[@]}" "${provenance[@]}" --access public "$pkg"
done

echo
echo "Published. Verify what a consumer gets:"
echo "  npm view @codesweep-ai/cs-lint"
