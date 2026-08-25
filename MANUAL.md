# The cs-lint manual

## Name

`cs-lint`: check that a repository, its documents and its claims hold together.

## Synopsis

```
cs-lint prose    [--stats] [--list] [--explain]
cs-lint refs     [--review] [--list] [--explain]
cs-lint surface  [--run] [--review] [--list] [--explain]
cs-lint oss      [--online] [--review] [--list] [--explain]
cs-lint manual
cs-lint version
cs-lint [command] --help
```

Every subcommand also accepts `--root` and `--verbose`.

## Description

cs-lint carries four linters over one repository. Each answers a different
question, and each reports findings with a rule identifier, a severity and an
address.

`prose` checks how the documents are written. `refs` checks that everything
they point at is still there. `surface` checks that the documented interface is
the real one, by comparing the documents against the tool's own help output,
the source, the build file, and the commands re-run now. `oss` checks what a
repository owes a reader before it is published.

The first two read the tracked tree and nothing else, so they answer before
anything is built. Every rule under `surface` asks the binary the repository
builds, so that one runs last.

Every check is mechanical and quotable. What needs judgement is left to the
review packs, because a linter that guesses produces noise, and noise gets
ignored.

The tool reads the repository the `--root` flag names, defaulting to the
working directory. It reads the tracked files, the git history, the tuning file
`.cs-lint.yaml`, and the binary the repository builds. It writes nothing.

## Commands

### prose

Check the prose in the repository's Markdown against the writing rules.

It reads every Markdown file at the repository root, and every one under
`docs/` or `doc/`. Code fences, tables, link definitions and raw HTML are
excluded throughout: they are not prose, and none of the rules are about them.

A document that numbers three or more requirements in bold is read as a
specification, and the rules written for ordinary prose are not applied to it.

| Flag | Meaning |
|---|---|
| `--list` | Print the files that would be checked, and stop. |
| `--stats` | Print the per-file measurements after the findings. |
| `--explain` | Print every rule, what it wants, and the guidance behind it. |

### refs

Check that everything the documents point at is still there.

Each of these is followed to what it claims to reach: a path a page names, a
section a citation points at, an issue an identifier promises a record for. A
document the router is supposed to list is followed the same way. Two
neighbours belong here for another reason. A block the reader is told to copy,
and a program the build needs, are both something the page hands them, and
neither can be followed to anything that exists.

Nothing here runs the tool, so a checkout with no binary built gets the whole
set.

| Flag | Meaning |
|---|---|
| `--review` | Print the review pack: the route a reader takes, which no pattern can judge. |
| `--list` | Print what was found to check, and stop. |
| `--explain` | Print every rule, what it wants, and why it exists. |

### surface

Check the documents against the binary, the code and the build.

It walks the tool's own help tree. It reads the source for the settings the tool
takes, and it re-runs the samples whose commands the tuning file declares safe.

Every rule needs the binary, so build first. Without one each rule reports a
skip rather than a pass, which is what lets a project run this without wiring a
build dependency.

| Flag | Meaning |
|---|---|
| `--run` | Print the ordered inventory of every command the documents tell a reader to run. |
| `--review` | Print the review pack for the rest. |
| `--list` | Print what was found to check, and stop. |
| `--explain` | Print every rule, what it wants, and why it exists. |

### oss

Check that the repository is in a shape it can be published in.

The rules cover the licence, the document set, and the leak scan over every
tracked file. They also cover the build, the release, what a stranger's clone
can do, the ledger where there is one, the git history, and the repository's own
settings on the forge.

| Flag | Meaning |
|---|---|
| `--online` | Ask the forge about the repository itself. Needs `gh`. |
| `--review` | Print the review pack: the questions a pattern cannot decide. |
| `--list` | Print what was found to check, and stop. |
| `--explain` | Print every rule, what it wants, and why it exists. |

### manual

Print this document, which is compiled into the binary. A machine with the tool
has the reference, with no checkout and no network.

### version

Print the version the binary was stamped with at build time.

## Options

| Option | Applies to | Meaning |
|---|---|---|
| `--root <path>` | every subcommand | The repository to check. Default `.`. |
| `--verbose` | every subcommand | Report what was skipped, and why. |
| `-h`, `--help` | every subcommand | Print the help for that command. |

## The rules

Each linter has families of its own, numbered from one. `--explain` is the
authority, and the tables below are the map.

### prose

| Rule | Severity | What it wants |
|---|---|---|
| `PROSE-101` | error | A glossary term is introduced where a document first uses it. |
| `PROSE-102` | error | Every sentence has a subject and a verb. |
| `PROSE-103` | error | A sentence carries one idea and stays under thirty words. |
| `PROSE-104` | error | No em-dash. |
| `PROSE-105` | error | A command runs only a script the document showed. |
| `PROSE-106` | error | The writing does not comment on itself. |
| `PROSE-107` | error | A sentence does not circle its own subject. |
| `PROSE-108` | error | The words this house has decided against. |
| `PROSE-109` | error | No word is written twice. |
| `PROSE-110` | error | An -ly adverb takes no hyphen. |
| `PROSE-111` | error | No merge conflict marker survives in the text. |
| `PROSE-112` | error | The README does not carry a section of negatives. |
| `PROSE-113` | error | Prose does not assert a number the repository counts itself. |

### refs

| Rule | Severity | What it wants |
|---|---|---|
| `REF-101` | error | Every repository path a document names exists. |
| `REF-102` | error | Every section citation resolves. |
| `REF-103` | error | Every section citation in the source resolves. |
| `REF-201` | warning | A block a reader copies names nothing they lack. |
| `REF-202` | error | Every tool the build needs is named in a document. |
| `REF-301` | warning | The manual answers the automated caller. |
| `REF-302` | error | The router names every document in the set. |
| `REF-303` | error | Every issue this repository cites has a record. |

The three families are the three kinds of promise. A hundred-block resolves a
reference to something in the tree. A two-hundred-block hands the reader
something they have to already have. A three-hundred-block is what the document
set owes the caller who arrives through it.

### surface

| Rule | Severity | What it wants |
|---|---|---|
| `SURF-101` | error | Every command a document names exists. |
| `SURF-102` | error | Every command the tool carries is documented. |
| `SURF-103` | warning | Every flag the tool carries is documented. |
| `SURF-104` | error | Every flag a document attributes to the tool exists. |
| `SURF-201` | error | Every environment variable the code reads is documented. |
| `SURF-202` | warning | Every environment variable a document names is read. |
| `SURF-301` | error | A sample output is what the command prints today. |
| `SURF-302` | warning | A version a document quotes is the version it ships. |
| `SURF-303` | error | The manual the binary carries is the manual in the tree. |

The families are the three things a tool tells you about itself: its commands
and flags, the settings it reads, and what it prints.

### oss

The readiness rules run from `OSS-1xx` to `OSS-8xx`. Run `cs-lint oss
--explain` for the list.

## Configuration

Every knob lives in `.cs-lint.yaml` at the repository root. A repository with
nothing to tune can leave the file out and gets the defaults. A key the schema
does not define fails the run, because a knob that silently does nothing is the
failure this format is most prone to.

The file has two top-level sections. `docs` holds the document set and one
block per documentation linter. `oss` holds the readiness knobs.

### docs

| Key | Default | Effect |
|---|---|---|
| `documents` | the five documents | The set `refs` and `surface` read the claims from. |
| `extraDocs` | empty | The standalone pages the set adds. |

The set sits above the two blocks that read it. A second copy drifts, and the
two halves would then disagree about which pages this repository publishes.

### docs.prose

| Key | Default | Effect |
|---|---|---|
| `skipExtra` | empty | Directories of fixtures, corpora or generated Markdown to leave out. |
| `glossary` | empty | Terms a reader cannot infer. An empty list disables the most valuable check. |
| `lowercaseStarters` | empty | Words that legitimately start a sentence in lower case. |
| `projectVerbs` | empty | Verbs the shared list does not carry. |
| `countable` | empty | Things this repository counts for itself. An empty list disables the check. |
| `terms` | empty | Pattern to advice, added to the declined-terms table. |
| `termsProse` | empty | The same, for documents that are not specs. |

### docs.refs

| Key | Default | Effect |
|---|---|---|
| `placeholderOK` | empty | Placeholder paths a block may name on purpose. |
| `prereqOK` | empty | Build tools no document has to name. |
| `markdownSkip` | empty | Path prefix to why the Markdown under it makes no claim about this repository. |
| `citationSkip` | empty | Path prefix to why a section number written there is not a citation. |
| `agentSection` | `Notes for agents` | The manual heading addressed to automated callers. |
| `allow` | empty | Rule identifier to the reason it is waived. |

### docs.surface

| Key | Default | Effect |
|---|---|---|
| `tool` | inferred | The command name. |
| `toolPath` | `bin/<tool>` | The binary this checkout builds, preferred over one on the path. |
| `envPrefix` | from the tool name | The variable prefix this tool reads. |
| `envInternal` | empty | Variable to why it is deliberately undocumented. |
| `safeVerbs` | empty | Verbs a sample check may re-run. An empty list disables that rule. |
| `sampleSkip` | empty | Command to why its sample cannot reproduce here. |
| `sourceSkip` | empty | Path prefix to why the source under it is not this tool's. |
| `allow` | empty | Rule identifier to the reason it is waived. |

Two of these are read by the reference rules as well. `tool` tells `REF-202`
which name in a build file is the tool itself rather than a program to install
first. `sourceSkip` tells `REF-303` which trees hold somebody else's code. Both
belong to the tool rather than to either linter, and a second copy of either
would let the two halves disagree.

### oss

| Key | Default | Effect |
|---|---|---|
| `project` | inferred | The command this repository ships. |
| `githubRepo` | from the remote | The `owner/name` this repository is published as. |
| `published` | `false` | Once true, a finding about a commit a remote already carries reports as a warning. |
| `docSet` | the six documents | The documents this repository carries. |
| `extraDocs` | empty | The standalone pages it adds, which the router must also name. |
| `homeAllow` | `user`, `you`, `name`, `runner` | Home names that are a placeholder rather than a person. |
| `emailAllow` | empty | Mail domains that are documentation addresses. |
| `skipPaths` | empty | Path prefix to the reason the scans skip it. |
| `allow` | empty | Rule identifier to the reason it is waived. |
| `binaryOK` | common asset types | Extensions a scan may skip as known binary assets. |
| `requiredTargets` | `build test check docs oss clean` | Task-runner targets that must exist. |
| `expectedTargets` | `help install uninstall fmt fmt-check vet lint` | The rest of the family's vocabulary. |

Every waiver takes a reason rather than a bare rule identifier, and that reason
is printed with the finding it waives. A waiver nobody can review is a rule
deleted in private.

A waiver naming a rule its linter does not carry stops the run with exit 2. An
identifier that matches nothing is a rule switched off in private, which is
exactly what requiring a reason exists to prevent.

## Migrating from the claims linter

`cs-lint walkthrough` was split in two. The rules that ask the binary are
`cs-lint surface`, and the rules that resolve a reference are `cs-lint refs`.
`cs-lint docs` was the prose linter, and is now `cs-lint prose`. Calling
`cs-lint walkthrough` exits 2 and names both successors.

Move a `walkthrough:` block to `docs.refs` and `docs.surface`, move the keys
that sat directly under `docs:` to `docs.prose`, and hoist the document set to
`docs.documents`. A file in the old shape fails the run with the new location
named.

Every rule kept its meaning, and only the identifier changed:

| Was | Is | Was | Is |
|---|---|---|---|
| `DOC-101` … `DOC-113` | `PROSE-101` … `PROSE-113` | `WALK-302` | `REF-101` |
| `WALK-101` | `SURF-101` | `WALK-303` | `REF-102` |
| `WALK-102` | `SURF-102` | `WALK-304` | `REF-103` |
| `WALK-103` | `SURF-103` | `WALK-301` | `REF-201` |
| `WALK-104` | `SURF-104` | `WALK-501` | `REF-202` |
| `WALK-201` | `SURF-201` | `WALK-601` | `REF-301` |
| `WALK-202` | `SURF-202` | `WALK-602` | `REF-302` |
| `WALK-401` | `SURF-301` | `WALK-603` | `REF-303` |
| `WALK-402` | `SURF-302` | | |
| `WALK-403` | `SURF-303` | | |

A waiver still naming an old identifier stops the run and names the new one.

## Files

| Path | Read or written | What it is |
|---|---|---|
| `.cs-lint.yaml` | read | The knobs for this repository. |
| `.leakterms` | read | Terms no pattern can infer. Absent by default, and gitignored when present. |
| `bin/<tool>` | read | The binary the interface linter interrogates. |
| the tracked files | read | What the scans cover, from `git ls-files`. |
| the git history | read | What the history rules read. |

cs-lint writes nothing. Every subcommand is read-only.

## Environment

cs-lint reads two variables, and neither is a setting:

| Variable | Effect |
|---|---|
| `USER` | The name the leak scan looks for, so the person running the check is checked for without their name being written down. |
| `LOGNAME` | The same, where `USER` is not set. |

No environment variable changes what a rule decides. A check that behaves
differently on two machines is a check nobody can act on.

## Exit status

| Code | Meaning |
|---|---|
| `0` | No rule reported an error. Warnings and skips may have been printed. |
| `1` | A rule reported an error. |
| `2` | The tool could not run: an unreadable tuning file, a waiver naming no rule, an unknown flag, or an unknown subcommand. |

The difference between 1 and 2 is the one a gate needs: the first is a finding
in the repository, and the second is a broken build.

## Diagnostics

**`cs-lint: .cs-lint.yaml: yaml: unmarshal errors: ... field proejct not found`**

A key the schema does not define. Check the spelling against the configuration
tables above. The run stops rather than continuing with a knob that does
nothing.

**``cs-lint: .cs-lint.yaml: `walkthrough:` was split in two``**

A tuning file written before the split. See the migration table above.

**`cs-lint: docs.refs.allow: WALK-302 is now REF-101`**

A waiver still naming a rule identifier from before the split. Rewrite it with
the new identifier, under the section that carries the rule.

**`cs-lint: verb list: error parsing regexp: ...`**
**`cs-lint: countable list: error parsing regexp: ...`**

A `projectVerbs` entry, or a `countable` one, is not a valid regular
expression. Each entry is a pattern fragment, so `mints?` is valid and `mint(`
is not.

**`OSS-306 cannot be read as text, so its contents were never checked`**

A tracked file that is neither valid text nor a declared binary asset. Remove
it, or add its extension to `binaryOK` if it is a legitimate asset. A file
nobody can inspect must never be reported as clean.

**A finding about the history, reported twice at two severities**

The rules that read past commits split their findings by reach. A commit no
remote in this clone carries is still yours to amend, so that half is an error
whatever `published` says. A commit a remote already has costs a rewrite of
every clone somebody else made, so that half follows `published`. A clone that
has a remote but has never fetched from it cannot tell the two apart, and there
`published` decides the whole finding.

**`OSS-702 ... commit subjects open with a category label`**

The subject opens with `feat:`, `fix(cli):`, `[docs]` or another category
label. The category is already in the diff, and the subject is the one line a
reader has for what the change does. Run `git commit --amend` on a commit you
have not pushed.

Unlike the leak scans over the history, this one stays an error after
publication: those describe what is already out, and this one describes what
the next contributor copies. A published history that already carries labels
cannot be rewritten, so waive `OSS-702` under `oss.allow` with that as the
reason.

**`OSS-502 a path outside the repository`, pointing at a file under `scripts/`**

A script or a build file names a `../` path that does not resolve. Where the
path is a real dependency on a sibling checkout, vendor it or publish it: a
stranger's clone is one directory. Where it is prose, such as an example inside a comment or a docstring,
declare the file in `oss.skipPaths` with the reason:

```yaml
oss:
  skipPaths:
    "scripts/lint-": "the vendored linters carry the patterns they search for"
```

A repository still vendoring a linter of its own is the common case. cs-lint
has no such file to exclude for itself, and it excludes nothing silently: a
skip that is not declared is a scan nobody can review.

**`SURF-101 ... is documented and the binary does not have it`**

A document names a command the tool no longer carries. Either the document is
stale, or `bin/<tool>` is older than the source. Rebuild before you edit.

**`surface: ... skipped`, with `no <tool> binary to ask`**

Every rule in this linter needs the binary. Run `make build` first; the
`surface` target does this for you.

**`SURF-301 ... skipped`, with `no safeVerbs configured`**

The sample check re-runs nothing until the tuning file declares which verbs are
safe to run. Every one has to be read-only, offline and safe in a checkout,
because they run on every gate.

**`the check itself failed: ...`**

A rule raised an error while running. The rest of the run continues, because
one broken check must not hide the findings of the others. This is a bug in
cs-lint; report it with the repository that provoked it.

## Notes for agents

Every subcommand is **non-interactive**. Nothing prompts, waits for input, or
opens an editor. One flag runs another program, and it is opt-in: `oss --online`
runs `gh`. Without it, cs-lint makes no network call.

Every subcommand is **read-only**. cs-lint never edits the repository. The
review packs are documents on standard output, so redirect one wherever you want
it:

```bash
cs-lint oss --review > /tmp/review.md
```

**Exit status is the machine-readable result.** 0 is a clean run, 1 reports
findings, and 2 says the tool could not run. Read the code rather than parsing
the text.

**The output is line-oriented.** Each finding is one line of
`RULE  SEVERITY  MESSAGE [ADDRESS]`, optionally followed by an indented quote.
The summary line is last.

**`--explain` is the reference for the rules**, and it is what to read before
deciding a finding is wrong. `--list` says what a run would cover, which is
worth checking before trusting a clean report.

**`surface --run` is the checklist** a reader works down. Every line names the
document, the line and the command, so a result can be recorded against a
stable address rather than a paraphrase.

**Order the gates by what they need.** `prose` and `refs` read the tracked tree
and nothing else, so they run first and answer in seconds. `oss` asks git for
the history. `surface` reads the binary the repository builds, so run
`make build` before driving it directly.

## Examples

Check everything, from a clean clone:

```bash
git clone https://github.com/codesweep-ai/lint /tmp/lint
cd /tmp/lint
make check
```

Check one repository's prose, and see what the rules are:

```bash
cs-lint --root ~/code/my-project prose
cs-lint prose --explain
```

Resolve every reference the documents make, which needs no build:

```bash
cs-lint refs
cs-lint refs --list
```

Find out what a readiness run would cover before trusting a clean report:

```bash
cs-lint oss --list
cs-lint oss --verbose
```

Walk the documented commands, then check the interface:

```bash
make build
cs-lint surface --run
cs-lint surface
```

Write the judgement half to a file:

```bash
cs-lint oss --review > /tmp/review.md
```

## See also

- [README.md](README.md) · what this is, and how to run it.
- [INSTALL.md](INSTALL.md) · how to get the tool.
- [SPEC.md](SPEC.md) · what the behaviour must be, and what is left open.
- [CONTRIBUTING.md](CONTRIBUTING.md) · how to work on it.
