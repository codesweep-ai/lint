#!/usr/bin/env node
// Turn one goreleaser build into the packages npm publishes.
//
// Five come out: four that each carry a single binary and declare the one
// platform it runs on, and the wrapper that depends on all four optionally and
// picks the right one at run time. Nothing here is committed. The binaries are
// goreleaser's, the version is the tag's, and both arrive at release time, so
// generating the packages is cheaper than keeping four hand-written copies of
// the same file honest.
//
//   goreleaser release --snapshot --clean     # or a real tagged release
//   node npm/build.mjs                        # -> npm/dist/
//
// Then publish what it wrote. `npm publish` is deliberately not run from here:
// this script is safe to run on any change, and a script that publishes is not.

import { readFileSync, writeFileSync, mkdirSync, copyFileSync, rmSync, chmodSync } from "node:fs";
import { execFileSync } from "node:child_process";
import { join, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const NPM = dirname(fileURLToPath(import.meta.url));
const ROOT = join(NPM, "..");
const DIST = join(ROOT, "dist");
const OUT = join(NPM, "dist");

// Go's names for a platform on the left, npm's on the right. npm reads `os` and
// `cpu` to decide what to install, so these strings are what keep a macOS
// laptop from downloading a Linux binary.
const PLATFORMS = {
  "darwin/amd64": { os: "darwin", cpu: "x64", suffix: "darwin-x64" },
  "darwin/arm64": { os: "darwin", cpu: "arm64", suffix: "darwin-arm64" },
  "linux/amd64": { os: "linux", cpu: "x64", suffix: "linux-x64" },
  "linux/arm64": { os: "linux", cpu: "arm64", suffix: "linux-arm64" },
};

// npm will not accept a leading v, and refuses anything that is not semver.
// goreleaser's snapshot version is a commit description rather than a version,
// so a snapshot has to say what it wants to be called.
// The version a --dev build carries: the one the binary reports about itself.
//
// Go stamps a pseudo-version into every build — the commit's UTC timestamp,
// then its hash — and `cs-lint version` prints it. Reusing it means the package
// on the registry and the binary inside it answer the same string, so a finding
// from a dev build names a version that can be looked up as a commit. It is
// valid semver, it sorts chronologically because the timestamp leads, and it
// carries no release number to be wrong about.
//
// Read out of the binary rather than derived from git, because the binary is
// what will be published, and a version computed beside it is a version that
// can disagree with it.
function devVersion() {
  const host = `${{ linux: "linux", darwin: "darwin" }[process.platform]}/${
    { x64: "amd64", arm64: "arm64" }[process.arch]
  }`;
  const binary = BINARIES.get(host);
  if (!binary) fail(`dist/ has no binary for this machine (${host}), so none can be asked its version`);

  const out = execFileSync(binary, ["version"], { encoding: "utf8" });
  const m = out.match(/v?(\d+\.\d+\.\d+[0-9A-Za-z.+-]*)/);
  if (!m) fail(`could not read a version out of \`cs-lint version\`, which printed: ${out.trim()}`);

  // Go marks a binary built from a modified tree with +dirty, and npm accepts
  // build metadata only to discard it: +dirty would publish under the clean
  // commit's version and take the name the committed build needs. A prerelease
  // identifier survives, so -dirty is what a dirty tree gets, as it is for the
  // image tags in cs-sandbox. It sorts after the commit it came from, nothing
  // ever publishes one, and it says plainly that this is not the published
  // build for that revision.
  const [pseudo, metadata] = m[1].split("+");
  return metadata === "dirty" ? `${pseudo}-dirty` : pseudo;
}

function version() {
  const override = process.env.CS_LINT_NPM_VERSION;
  if (!override && process.argv.includes("--dev")) return devVersion();

  const meta = JSON.parse(readFileSync(join(DIST, "metadata.json"), "utf8"));
  const raw = (override ?? meta.version).replace(/^v/, "");
  if (!/^\d+\.\d+\.\d+(-[0-9A-Za-z.-]+)?(\+[0-9A-Za-z.-]+)?$/.test(raw)) {
    fail(
      `"${raw}" is not a version npm will accept.\n` +
        `A tagged release supplies one. For a dev build off any commit, use --dev,\n` +
        `which takes the version the binary reports. To name one yourself:\n` +
        `  CS_LINT_NPM_VERSION=0.1.0-snapshot.1 node npm/build.mjs`,
    );
  }
  return raw;
}

function fail(message) {
  console.error(`npm/build.mjs: ${message}`);
  process.exit(1);
}

// Every binary goreleaser built, keyed by platform.
function binaries() {
  let artifacts;
  try {
    artifacts = JSON.parse(readFileSync(join(DIST, "artifacts.json"), "utf8"));
  } catch {
    fail(`no dist/artifacts.json. Run \`goreleaser release --snapshot --clean\` first.`);
  }
  const found = new Map();
  for (const a of artifacts) {
    if (a.type !== "Binary") continue;
    found.set(`${a.goos}/${a.goarch}`, join(ROOT, a.path));
  }
  return found;
}

const BINARIES = binaries();
const VERSION = version();

// A wrapper published against platform packages that were never built installs
// cleanly and then fails at run time on the platform that is missing, which is
// the one failure this whole arrangement exists to avoid. So a partial build
// stops here instead.
const missing = Object.keys(PLATFORMS).filter((p) => !BINARIES.has(p));
if (missing.length) {
  fail(
    `dist/ has no binary for ${missing.join(", ")}.\n` +
      `A single-target build cannot produce the package set. Use:\n` +
      `  goreleaser release --snapshot --clean`,
  );
}

rmSync(OUT, { recursive: true, force: true });

// Apache 2.0 section 4(d) obliges every redistributor to carry NOTICE, and an
// npm package is a redistribution exactly as a release archive is.
function carryLicence(dir) {
  for (const f of ["LICENSE", "NOTICE"]) copyFileSync(join(ROOT, f), join(dir, f));
}

function writeJSON(path, value) {
  writeFileSync(path, `${JSON.stringify(value, null, 2)}\n`);
}

const repository = {
  type: "git",
  url: "git+https://github.com/codesweep-ai/lint.git",
};

// The four platform packages. Each is a binary, a licence and a package.json
// that says which machine it is for: no code, and nothing to run on install.
const platformNames = [];
for (const [target, { os, cpu, suffix }] of Object.entries(PLATFORMS)) {
  const name = `@codesweep-ai/cs-lint-${suffix}`;
  platformNames.push(name);

  const dir = join(OUT, `cs-lint-${suffix}`);
  mkdirSync(join(dir, "bin"), { recursive: true });

  const binary = join(dir, "bin", "cs-lint");
  copyFileSync(BINARIES.get(target), binary);
  // npm preserves the executable bit, but the copy inherits whatever goreleaser
  // left and a package whose binary is not executable fails with EACCES.
  chmodSync(binary, 0o755);

  writeJSON(join(dir, "package.json"), {
    name,
    version: VERSION,
    description: `The cs-lint binary for ${os} ${cpu}. Installed by @codesweep-ai/cs-lint; not used directly.`,
    license: "Apache-2.0",
    repository,
    homepage: "https://github.com/codesweep-ai/lint#readme",
    // npm installs an optional dependency only where these match, which is what
    // makes the wrapper's four dependencies cost one download.
    os: [os],
    cpu: [cpu],
    // access only. Provenance is a property of the machine doing the publish
    // rather than of the package, and asking for it here fails a publish from
    // anywhere without an OIDC token: npm/publish.sh adds the flag in CI.
    publishConfig: { access: "public" },
    // No `exports`, on purpose: the wrapper resolves `<name>/bin/cs-lint`, and
    // an exports map would have to list that path to allow it.
    files: ["bin", "README.md", "LICENSE", "NOTICE"],
  });

  writeFileSync(
    join(dir, "README.md"),
    `# ${name}\n\n` +
      `The cs-lint binary for ${os} ${cpu}.\n\n` +
      `This package is one of four, and installing it directly is not the way in. ` +
      `Install [\`@codesweep-ai/cs-lint\`](https://www.npmjs.com/package/@codesweep-ai/cs-lint), ` +
      `which depends on all four optionally and resolves the one this machine can run.\n`,
  );

  carryLicence(dir);
}

// The wrapper, taken from the committed source with the version stamped into it
// and into every dependency on a platform package. The pins are exact: a
// wrapper that accepted a range could pair itself with a binary built from
// different source.
const wrapperDir = join(OUT, "cs-lint");
mkdirSync(join(wrapperDir, "bin"), { recursive: true });

const wrapper = JSON.parse(readFileSync(join(NPM, "cs-lint", "package.json"), "utf8"));
wrapper.version = VERSION;
wrapper.optionalDependencies = Object.fromEntries(
  platformNames.sort().map((name) => [name, VERSION]),
);
writeJSON(join(wrapperDir, "package.json"), wrapper);

for (const f of ["index.mjs", "README.md"]) {
  copyFileSync(join(NPM, "cs-lint", f), join(wrapperDir, f));
}
copyFileSync(join(NPM, "cs-lint", "bin", "cs-lint.mjs"), join(wrapperDir, "bin", "cs-lint.mjs"));
chmodSync(join(wrapperDir, "bin", "cs-lint.mjs"), 0o755);
carryLicence(wrapperDir);

console.log(`cs-lint ${VERSION} -> npm/dist/`);
for (const name of [...platformNames, "@codesweep-ai/cs-lint"].sort()) {
  console.log(`  ${name}`);
}
console.log(`\nPublish with:\n  npm/publish.sh`);
