# @codesweep-ai/cs-lint

Four linters over one repository: how its documents are written, whether
everything they point at is still there, whether the interface they describe is
the real one, and what a published repository owes a reader.

This package carries the [cs-lint](https://github.com/codesweep-ai/lint) binary
for npm projects. The tool is written in Go and ships as a static binary, so
nothing here needs a Go toolchain: npm installs the one binary your machine
runs, and nothing is compiled, downloaded or written when you install it.

## Install

```bash
npm install --save-dev @codesweep-ai/cs-lint
```

Or run it once against the repository you are standing in:

```bash
npx @codesweep-ai/cs-lint prose
```

## Use

Each linter is a verb, and each reads the repository and writes nothing:

```bash
cs-lint prose      # how the documents are written
cs-lint refs       # whether every reference in them resolves
cs-lint oss        # what a published repository owes a reader
cs-lint surface    # whether the documented interface is the real one
```

The value is in the gate rather than the command you remember to type, so put
them in the one script a contributor already runs before pushing:

```json
{
  "scripts": {
    "docs:prose": "cs-lint prose",
    "docs:refs": "cs-lint refs",
    "oss": "cs-lint oss",
    "check": "npm run docs:prose && npm run docs:refs && npm run oss && npm test"
  }
}
```

`prose` and `refs` read the tracked tree and nothing else, so they answer in
seconds and belong early, before anything is built.

Every run exits 0 when it found nothing, 1 when it found something, and 2 when
it could not run at all. A gate reads all three.

## Reading further

The [repository](https://github.com/codesweep-ai/lint) says what each linter is
for. [MANUAL.md](https://github.com/codesweep-ai/lint/blob/main/MANUAL.md) has
the full surface: every rule, every flag, and every key of the `.cs-lint.yaml`
a project writes when it has something to tune.
[INSTALL.md](https://github.com/codesweep-ai/lint/blob/main/INSTALL.md) covers
the programs a few checks skip without, and the routes a project with a Go
toolchain can take instead.

## Platforms

macOS and Linux, on both Intel and Apple silicon. The binaries ship as four
packages, each marked with the platform it is for and listed as an optional
dependency of this one, so npm installs the single package that matches and
skips the other three.

If you see a message saying a platform package is missing, npm has dropped an
optional dependency, which a clean reinstall fixes:

```bash
rm -rf node_modules package-lock.json && npm install
```

## License

[Apache-2.0](https://github.com/codesweep-ai/lint/blob/main/LICENSE).
