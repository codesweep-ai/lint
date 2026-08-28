// Where the binary for this machine is.
//
// The binaries themselves ship as one package per platform, each marked with
// the `os` and `cpu` it is for and listed as an optional dependency of this
// one. npm installs the single package that matches and skips the rest, so a
// machine downloads one binary rather than four. This file is the lookup that
// finds it again at run time.
//
// It is exported rather than kept private because a project that wants to run
// the linter from its own script needs the path, and reaching into another
// package's directory to guess it is what this export exists to prevent.

import { createRequire } from "node:module";

const require = createRequire(import.meta.url);

// goreleaser names its targets in Go's vocabulary and npm filters installs with
// its own, so the two have to be mapped. The table is written out rather than
// derived from a rule, because a platform nobody built for must be a lookup
// miss that names the ones that exist, not a package name assembled from parts
// that resolves to nothing.
const PACKAGES = {
  "darwin arm64": "@codesweep-ai/cs-lint-darwin-arm64",
  "darwin x64": "@codesweep-ai/cs-lint-darwin-x64",
  "linux arm64": "@codesweep-ai/cs-lint-linux-arm64",
  "linux x64": "@codesweep-ai/cs-lint-linux-x64",
};

// Raised when this platform has no build. Distinct from the class below,
// because the two have different answers: this one is a porting request, and
// that one is a broken install.
export class UnsupportedPlatformError extends Error {
  constructor(platform, arch) {
    super(
      `cs-lint has no build for ${platform}/${arch}.\n` +
        `Built platforms: ${Object.keys(PACKAGES).join(", ").replace(/ (?=\w+,|\w+$)/g, "/")}.\n` +
        `Ask for one at https://github.com/codesweep-ai/lint/issues, or build from ` +
        `a clone: https://github.com/codesweep-ai/lint/blob/main/INSTALL.md`,
    );
    this.name = "UnsupportedPlatformError";
    this.platform = platform;
    this.arch = arch;
  }
}

// Raised when the platform is supported but its package is not on disk. Nearly
// always npm rather than the platform: an install that was copied between
// machines, or one that hit npm's long-standing bug where an existing
// node_modules loses optional dependencies. Both are fixed the same way, so the
// message says so rather than describing the cause.
export class MissingBinaryError extends Error {
  constructor(pkg, cause) {
    super(
      `cs-lint is installed, but ${pkg} is not.\n` +
        `That package carries the binary for this platform and installs with ` +
        `cs-lint itself, so this usually means a node_modules built on another ` +
        `platform, or npm dropping an optional dependency.\n` +
        `Reinstall from a clean state:\n` +
        `  rm -rf node_modules package-lock.json && npm install\n` +
        `In CI, prefer \`npm ci\` over a restored node_modules cache.`,
      { cause },
    );
    this.name = "MissingBinaryError";
    this.package = pkg;
  }
}

// The absolute path to the cs-lint binary for this machine.
//
// CS_LINT_BINARY overrides the lookup entirely. That is what lets this
// repository run the npm packaging against the binary it just built, rather
// than against the last one published, so the packaging is tested by the same
// gate as everything else.
export function binaryPath() {
  const override = process.env.CS_LINT_BINARY;
  if (override) return override;

  const pkg = PACKAGES[`${process.platform} ${process.arch}`];
  if (!pkg) throw new UnsupportedPlatformError(process.platform, process.arch);

  // Resolved through require rather than joined onto a path, so it is found
  // wherever the installer put it: nested, hoisted, or linked out of a store by
  // pnpm or Yarn.
  try {
    return require.resolve(`${pkg}/bin/cs-lint`);
  } catch (cause) {
    throw new MissingBinaryError(pkg, cause);
  }
}

export default binaryPath;
