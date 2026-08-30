# @codesweep-ai/cs-lint

> **Various linters: doc style, doc correctness, open-source readiness, and more.**

[![CI](https://github.com/codesweep-ai/lint/actions/workflows/ci.yml/badge.svg)](https://github.com/codesweep-ai/lint/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](https://github.com/codesweep-ai/lint/blob/main/LICENSE)
![Rules](https://img.shields.io/badge/rules-105-informational)

`cs-lint` is a linter for repositories. It has four commands:

- **`cs-lint prose`** checks how the documentation is written.
- **`cs-lint refs`** checks that everything the documentation points at is
  still there.
- **`cs-lint surface`** checks that the documented interface is the real one.
- **`cs-lint oss`** checks that the repository has what an open-source project
  needs.

Each one prints a rule number, what is wrong, and the file and line to look at.
Each exits non-zero when it finds something, so you can run it in CI.

The linter is written in Go, and packaged here for npm projects.

## Quickstart

```bash
npm install --save-dev @codesweep-ai/cs-lint

cd ~/code/my-project
cs-lint prose          # how the documents are written
cs-lint refs           # whether everything they point at is there
cs-lint oss            # what a published repository owes a reader
cs-lint surface        # whether the documented interface is the real one
```

A repository with nothing to tune needs no configuration. To tune one, write
`.cs-lint.yaml` at its root:

```yaml
docs:
  documents: [README.md, MANUAL.md, SPEC.md]   # read by refs and surface

  prose:
    glossary: [cassette, ruleset]     # terms a reader cannot infer
    lowercaseStarters: [my-tool]      # the command name, which starts sentences

  refs:
    placeholderOK: [my-project]       # paths a page leaves to the reader

  surface:
    tool: my-tool
    toolPath: bin/my-tool
    safeVerbs: [version, status]      # read-only verbs a sample check may re-run

oss:
  project: my-tool
  githubRepo: acme/my-tool
```

Then wire it into the one command a contributor already runs:

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

## Docs

The documentation lives in the [codesweep-ai/lint](https://github.com/codesweep-ai/lint)
GitHub repository, and none of it ships in this package.

- [INSTALL.md](https://github.com/codesweep-ai/lint/blob/main/INSTALL.md) · how to get the tool, and the setup it needs once
- [MANUAL.md](https://github.com/codesweep-ai/lint/blob/main/MANUAL.md) · the full surface: commands, options, settings, exit codes
- [SPEC.md](https://github.com/codesweep-ai/lint/blob/main/SPEC.md) · what the behaviour must be, and what is left open
- [CONTRIBUTING.md](https://github.com/codesweep-ai/lint/blob/main/CONTRIBUTING.md) · conventions, and the rituals a diff does not show
- [AGENTS.md](https://github.com/codesweep-ai/lint/blob/main/AGENTS.md) · where an agent looks first

## Contributing

Read [CONTRIBUTING.md](https://github.com/codesweep-ai/lint/blob/main/CONTRIBUTING.md).
It applies to coding agents as well as to people.

## License

[Apache-2.0](https://github.com/codesweep-ai/lint/blob/main/LICENSE).
