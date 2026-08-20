# The cs-lint manual

## Name

`cs-lint`: check that a repository, its documents and its claims hold together.

## Synopsis

```
cs-lint docs         [--stats] [--list] [--explain]
cs-lint oss          [--online] [--review] [--list] [--explain]
cs-lint walkthrough  [--run] [--review] [--list] [--explain]
cs-lint manual
cs-lint version
cs-lint [command] --help
```

Every subcommand also accepts `--root` and `--verbose`.

## Description

cs-lint carries three linters over one repository. Each answers a different
question, and each reports findings with a rule identifier, a severity and an
address.

`docs` checks how the documents are written. `oss` checks what a repository
owes a reader before it is published. `walkthrough` checks whether those
documents still describe the software, by comparing them against the tool's own
help output, the source, the build file, and the commands re-run now.

Every check is mechanical and quotable. What needs judgement is left to the
review packs, because a linter that guesses produces noise, and noise gets
ignored.

The tool reads the repository the `--root` flag names, defaulting to the
working directory. It reads the tracked files, the git history, the tuning file
`.cs-lint.yaml`, and the binary the repository builds. It writes nothing.

## Commands

### docs

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

### walkthrough

Check the documents against the binary, the code and the build.

It walks the tool's own help tree. It reads the source for the settings the tool
takes, and the build file for the programs it shells out to. It then re-runs the
samples whose commands the tuning file declares safe.

| Flag | Meaning |
|---|---|
| `--run` | Print the ordered inventory of every command the documents tell a reader to run. |
| `--review` | Print the review pack for the rest. |
| `--list` | Print what was found to check, and stop. |
| `--explain` | Print every rule, what it wants, and why it exists. |

`walk` is accepted as an alias.

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

## Configuration

Every knob lives in `.cs-lint.yaml` at the repository root. A repository with
nothing to tune can leave the file out and gets the defaults. A key the schema
does not define fails the run, because a knob that silently does nothing is the
failure this format is most prone to.

### docs

| Key | Default | Effect |
|---|---|---|
| `skipExtra` | empty | Directories of fixtures, corpora or generated Markdown to leave out. |
| `glossary` | empty | Terms a reader cannot infer. An empty list disables the most valuable check. |
| `lowercaseStarters` | empty | Words that legitimately start a sentence in lower case. |
| `projectVerbs` | empty | Verbs the shared list does not carry. |
| `countable` | empty | Things this repository counts for itself. An empty list disables the check. |
| `terms` | empty | Pattern to advice, added to the declined-terms table. |
| `termsProse` | empty | The same, for documents that are not specs. |

### oss

| Key | Default | Effect |
|---|---|---|
| `project` | inferred | The command this repository ships. |
| `githubRepo` | from the remote | The `owner/name` this repository is published as. |
| `published` | `false` | Once true, the history rules report as warnings. |
| `docSet` | the six documents | The documents this repository carries. |
| `extraDocs` | empty | The standalone pages it adds, which the router must also name. |
| `homeAllow` | `user`, `you`, `name`, `runner` | Home names that are a placeholder rather than a person. |
| `emailAllow` | empty | Mail domains that are documentation addresses. |
| `skipPaths` | empty | Path prefix to the reason the scans skip it. |
| `allow` | empty | Rule identifier to the reason it is waived. |
| `binaryOK` | common asset types | Extensions a scan may skip as known binary assets. |
| `requiredTargets` | `build test check docs oss clean` | Task-runner targets that must exist. |
| `expectedTargets` | `help install uninstall fmt fmt-check vet lint` | The rest of the family's vocabulary. |

### walkthrough

| Key | Default | Effect |
|---|---|---|
| `tool` | inferred | The command name. |
| `toolPath` | `bin/<tool>` | The binary this checkout builds, preferred over one on the path. |
| `docs` | the five documents | The document set the claims are read from. |
| `extraDocs` | empty | The standalone pages the set adds. |
| `envPrefix` | from the tool name | The variable prefix this tool reads. |
| `envInternal` | empty | Variable to why it is deliberately undocumented. |
| `safeVerbs` | empty | Verbs a sample check may re-run. An empty list disables that rule. |
| `sampleSkip` | empty | Command to why its sample cannot reproduce here. |
| `placeholderOK` | empty | Placeholder paths a block may name on purpose. |
| `prereqOK` | empty | Build tools no document has to name. |
| `sourceSkip` | empty | Path prefix to why its settings are not this tool's. |
| `markdownSkip` | empty | Path prefix to why the Markdown under it makes no claim about this repository. |
| `citationSkip` | empty | Path prefix to why a section number written there is not a citation. |
| `agentSection` | `Notes for agents` | The manual heading addressed to automated callers. |
| `allow` | empty | Rule identifier to the reason it is waived. |

Every waiver takes a reason rather than a bare rule identifier, and that reason
is printed with the finding it waives. A waiver nobody can review is a rule
deleted in private.

## Files

| Path | Read or written | What it is |
|---|---|---|
| `.cs-lint.yaml` | read | The knobs for this repository. |
| `.leakterms` | read | Terms no pattern can infer. Absent by default, and gitignored when present. |
| `bin/<tool>` | read | The binary the walkthrough linter interrogates. |
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
| `2` | The tool could not run: an unreadable tuning file, an unknown flag, or an unknown subcommand. |

The difference between 1 and 2 is the one a gate needs: the first is a finding
in the repository, and the second is a broken build.

## Diagnostics

**`cs-lint: .cs-lint.yaml: yaml: unmarshal errors: ... field proejct not found`**

A key the schema does not define. Check the spelling against the configuration
tables above. The run stops rather than continuing with a knob that does
nothing.

**`cs-lint: verb list: error parsing regexp: ...`**

A `projectVerbs` or `countable` entry is not a valid regular expression. Each entry is a
pattern fragment, so `mints?` is valid and `mint(` is not.

**`OSS-306 cannot be read as text, so its contents were never checked`**

A tracked file that is neither valid text nor a declared binary asset. Remove
it, or add its extension to `binaryOK` if it is a legitimate asset. A file
nobody can inspect must never be reported as clean.

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

**`WALK-101 ... is documented and the binary does not have it`**

A document names a command the tool no longer carries. Either the document is
stale, or `bin/<tool>` is older than the source. Rebuild before you edit.

**`walkthrough: ... skipped`, with `no <tool> binary to ask`**

Half the claims checks need the binary. Run `make build` first; the
`walkthrough` target does this for you.

**`WALK-401 ... skipped`, with `no safeVerbs configured`**

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

**`walkthrough --run` is the checklist** a walkthrough works down. Every line
names the document, the line and the command, so a result can be recorded
against a stable address rather than a paraphrase.

**Building first matters.** `docs` asks for no binary and `oss` asks for git,
but `walkthrough` reads the binary the repository builds. Run `make build`
before driving it directly.

## Examples

Check everything, from a clean clone:

```bash
git clone https://github.com/codesweep-ai/lint /tmp/lint
cd /tmp/lint
make check
```

Check one repository's prose, and see what the rules are:

```bash
cs-lint --root ~/code/my-project docs
cs-lint docs --explain
```

Find out what a readiness run would cover before trusting a clean report:

```bash
cs-lint oss --list
cs-lint oss --verbose
```

Walk the documented commands, then check the claims:

```bash
make build
cs-lint walkthrough --run
cs-lint walkthrough
```

Write the judgement half to a file:

```bash
cs-lint oss --review > /tmp/review.md
```

## See also

- [`README.md`](README.md): what this is, and how to run it.
- [`INSTALL.md`](INSTALL.md): how to get the tool.
- [`SPEC.md`](SPEC.md): what the behaviour must be, and what is left open.
- [`CONTRIBUTING.md`](CONTRIBUTING.md): how to work on it.
