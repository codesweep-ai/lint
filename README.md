# lint

> **Various linters: doc style, doc correctness, open-source readiness, and more.**

[![CI](https://github.com/codesweep-ai/lint/actions/workflows/ci.yml/badge.svg)](https://github.com/codesweep-ai/lint/actions/workflows/ci.yml)
[![License](https://img.shields.io/badge/license-Apache--2.0-blue.svg)](LICENSE)
![Rules](https://img.shields.io/badge/rules-102-informational)
![Platforms](https://img.shields.io/badge/platform-Linux%20%C2%B7%20macOS-lightgrey)

`cs-lint` is a linter for repositories. It has four commands:

- **`cs-lint prose`** checks how the documentation is written.
- **`cs-lint refs`** checks that everything the documentation points at is
  still there.
- **`cs-lint surface`** checks that the documented interface is the real one.
- **`cs-lint oss`** checks that the repository has what an open-source project
  needs.

Each one prints a rule number, what is wrong, and the file and line to look at.
Each exits non-zero when it finds something, so you can run it in CI.

```
                        .cs-lint.yaml         the knobs, one file
                              │
      ┌───────────────┬───────┴───────┬───────────────┐
      ▼               ▼               ▼               ▼
 cs-lint prose   cs-lint refs   cs-lint oss   cs-lint surface
      │               │               │               │
   the prose      the paths      the tracked tree   the help tree
   in *.md        + the cites    + the git history  + the source
      │           + the router         │            + the commands, re-run
      ▼               ▼                ▼               ▼
   PROSE-1xx       REF-1xx…3xx     OSS-1xx…8xx     SURF-1xx…3xx
      └───────────────┴───────┬───────┴───────────────┘
                              ▼
              rule  severity  message  [file:line]
                 exit 0 clean · 1 findings · 2 broken
```

The first two read the tracked tree and nothing else, so they run before
anything is built. `cs-lint surface` asks the binary the repository builds, so
it runs after the build.

## Quickstart

```bash
go install github.com/codesweep-ai/lint/cmd/cs-lint@latest

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

```make
docs: ; cs-lint prose && cs-lint refs
oss: ; cs-lint oss
surface: build ; cs-lint surface

check: fmt-check vet lint test docs oss surface
```

## The four linters

### `cs-lint prose`

Checks the writing in every Markdown file at the repository root and under
`docs/`. It reports sentences over thirty words, words the project has decided
against, a term used before anything explains it, and writing that comments on
itself. It also catches mechanical slips: a word written twice, an em-dash, a
merge marker left in the text.

Rules taken from a published style guide say which one, so you can look up the
reasoning rather than argue with the tool.

### `cs-lint refs`

Checks that everything the documentation points at resolves. It reports:

- a file path a page names that is no longer there
- a section reference, in a document or in a source comment, that points at
  nothing
- an issue identifier with no record behind it
- a document the router never names
- a block the reader is told to copy that opens on a path they were never given
- a program the build needs that no document tells anybody to install

Nothing here runs the tool, so this is the check a repository can run before it
builds.

### `cs-lint surface`

Checks that the documented interface is the real interface. It runs the tool's
own `--help`, walks the subcommands it finds, and compares all of it against
the documents. It reports:

- a command the docs name that the binary does not have
- a command the binary has that no document mentions
- a setting the code reads that no document lists
- example output that is no longer what the command prints
- a manual compiled into a binary that has gone stale against the file

The output check re-runs the commands you mark as safe to run, and compares
their output line by line.

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
reason is a value rather than a comment. A waiver naming a rule that does not
exist stops the run, because an identifier that matches nothing switches a rule
off and says nothing about it.

## Docs

- [INSTALL.md](INSTALL.md) · how to get the tool, and the setup it needs once
- [MANUAL.md](MANUAL.md) · the full surface: commands, options, settings, exit codes
- [SPEC.md](SPEC.md) · what the behaviour must be, and what is left open
- [CONTRIBUTING.md](CONTRIBUTING.md) · conventions, and the rituals a diff does not show
- [AGENTS.md](AGENTS.md) · where an agent looks first

## Contributing

Read [CONTRIBUTING.md](CONTRIBUTING.md), and run `make check` before you push.

## License

[Apache-2.0](LICENSE).
