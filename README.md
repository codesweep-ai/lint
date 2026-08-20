# lint

> **Various linters: doc style, doc correctness, open-source readiness, and more.**

[![CI](https://github.com/codesweep-ai/lint/actions/workflows/ci.yml/badge.svg)](https://github.com/codesweep-ai/lint/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
![Rules](https://img.shields.io/badge/rules-90-informational)
![Platforms](https://img.shields.io/badge/platform-Linux%20%C2%B7%20macOS-lightgrey)

`cs-lint` is a linter for repositories. It has three commands:

- **`cs-lint docs`** checks how the documentation is written.
- **`cs-lint walkthrough`** checks that the documentation matches the software.
- **`cs-lint oss`** checks that the repository has what an open-source project
  needs.

Each one prints a rule number, what is wrong, and the file and line to look at.
Each exits non-zero when it finds something, so you can run it in CI.

```
                  .cs-lint.yaml            the knobs, one file
                        │
      ┌─────────────────┼─────────────────┐
      ▼                 ▼                 ▼
  cs-lint docs      cs-lint oss    cs-lint walkthrough
      │                 │                 │
   the prose      the tracked tree     the help tree
   in *.md        + the git history    + the source
      │                 │              + the build file
      │                 │              + the commands, re-run
      ▼                 ▼                 ▼
   DOC-1xx           OSS-1xx…8xx       WALK-1xx…6xx
      └─────────────────┼─────────────────┘
                        ▼
              rule  severity  message  [file:line]
                 exit 0 clean · 1 findings · 2 broken
```

## Quickstart

```bash
go install github.com/codesweep-ai/lint/cmd/cs-lint@latest

cd ~/code/my-project
cs-lint docs           # how the documents are written
cs-lint oss            # what a published repository owes a reader
cs-lint walkthrough    # whether the documents still describe the software
```

A repository with nothing to tune needs no configuration. To tune one, write
`.cs-lint.yaml` at its root:

```yaml
docs:
  glossary: [cassette, ruleset]     # terms a reader cannot infer
  lowercaseStarters: [my-tool]      # the command name, which starts sentences

oss:
  project: my-tool
  githubRepo: acme/my-tool

walkthrough:
  tool: my-tool
  toolPath: bin/my-tool
  safeVerbs: [version, status]      # read-only verbs a sample check may re-run
```

Then wire it into the one command a contributor already runs:

```make
docs: ; cs-lint docs
oss: ; cs-lint oss
walkthrough: build ; cs-lint walkthrough

check: fmt-check vet lint test docs oss walkthrough
```

## The three linters

### `cs-lint docs`

Checks the writing in every Markdown file at the repository root and under
`docs/`. It reports sentences over thirty words, words the project has decided
against, a term used before anything explains it, and writing that comments on
itself. It also catches mechanical slips: a word written twice, an em-dash, a
merge marker left in the text.

Rules taken from a published style guide say which one, so you can look up the
reasoning rather than argue with the tool.

### `cs-lint walkthrough`

Checks that the documentation still describes the software. It runs the tool's
own `--help`, walks the subcommands it finds, and compares all of it against
the documents. It reports:

- a command the docs name that the binary does not have
- a command the binary has that no document mentions
- a setting the code reads that no document lists
- a file path or a section reference that no longer exists
- example output that is no longer what the command prints

The last one re-runs the commands you mark as safe to run, and compares their
output line by line.

### `cs-lint oss`

Checks that the repository has what an open-source project needs before you
publish it:

- a licence file with the full text and a real copyright line
- the expected set of documents, with every link and heading reference working
- a build, a test command and a release that someone else can run
- no private material in any tracked file or in any past commit

Private material means home directories, email addresses, API keys and the
like. cs-lint searches for the *shape* of these rather than a list of real
names, so nothing private has to be written into the configuration.

## Review packs

Some questions need a person. Does this paragraph give away something private?
Was this concept ever explained? Is this claim about the software actually
true? cs-lint does not try to answer those. It prints a checklist of them
instead, each with the commands to gather the evidence first:

```bash
cs-lint oss --review > review.md
```

The result is a Markdown file. cs-lint prints it and stops there.

## Exceptions

A rule is turned off for one repository with a waiver, which is a rule
identifier and the reason it was traded away. The reason is required, and it is
printed with the finding:

```yaml
oss:
  allow:
    OSS-204: "the badge lives on the docs site, not in the README"
```

A waiver nobody can review is a rule deleted in private, which is why the
reason is a value rather than a comment.

## Docs

- [`INSTALL.md`](INSTALL.md): how to get the tool, and the setup it needs once.
- [`MANUAL.md`](MANUAL.md): the full surface: commands, options, settings, exit codes.
- [`SPEC.md`](SPEC.md): what the behaviour must be, and what is left open.
- [`CONTRIBUTING.md`](CONTRIBUTING.md): conventions, and the rituals a diff does not show.
- [`AGENTS.md`](AGENTS.md): where an agent looks first.

## Contributing

Read [`CONTRIBUTING.md`](CONTRIBUTING.md), and run `make check` before you push.

## License

[Apache-2.0](LICENSE).
