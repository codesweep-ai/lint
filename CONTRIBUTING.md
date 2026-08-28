# Contributing to cs-lint

Bug reports and pull requests are welcome. These rules apply to humans and
coding agents alike. If you are an agent working in this repository, read this
file before you change anything and follow it.

For a security issue, use GitHub's private vulnerability reporting on this
repository's Security tab, rather than opening a public issue.

## Submitting a change

File a bug or an idea as a GitHub issue on this repository. For a fix that
stands on its own, a pull request on its own is enough. For anything that adds
a rule, changes a report or moves a boundary, open an issue first, so the
design gets settled before you write it.

1. Fork the repository, and create a branch off `main`.
2. Make the change, with its test.
3. Run `make check`, which is the same gate CI runs.
4. Open a pull request against `main`, and say what the change does and why.

Expect comments rather than silence, and expect a small change to move
quickly. A reviewer asks whether the change keeps the design rules below,
whether a test fails without it, and where a reader would find it
documented.

By opening a pull request you agree that your contribution ships under the
[Apache 2.0 licence](LICENSE) this project is released under.

## Before you push

One command:

```bash
make ci
```

That is every gate the CI workflow has, on this machine and in the order the
workflow takes them, so a green run here is a green run there. `make check`
is the faster subset to keep beside you while you work, and `make ci` is the
one that has to pass.

No linter needs installing. The ones it shells out to are pinned Go tools,
built from the module cache the first time you run them: `golangci-lint`,
`deadcode` and `actionlint`. `make versions` prints the version of each.

Moving a pin is an edit to `go.mod`, or to `go.golangci.mod` for
`golangci-lint`. A linter release reaches you when you ask for it, not on an
unrelated pull request.

`goreleaser` is the one program still expected on the PATH. `make ci`
validates the release manifest with it, and `make build` falls back to
`go build` where it is absent.

## Design rules

Your change has to keep these. Each one names the test or the review that
holds it.

**A rule never guesses.** Every check compares a document against something
that cannot lie: the tool's own help output, the source, the build file, the
git history, or the command re-run now. Anything that needs judgement goes
in a review pack instead.

**The tool carries no project knowledge.** No repository name, no glossary,
no path belongs in a rule. Everything that differs between repositories
lives in `.cs-lint.yaml`, so a fix to a check reaches every project at once.

**Every exception is reviewable.** A waiver is a rule identifier and the
reason it was traded away, and the reason is printed with the finding. The
configuration schema makes the reason the value rather than an optional
comment.

**A check that could not run reports a skip**, never a pass. The tests
assert a skip wherever a tool is absent.

**One broken check does not hide the rest.** A rule that fails while running
is reported as that rule failing, and the others still run. A test panics
inside a rule and asserts the run survives.

**The tool writes nothing.** Every subcommand is read-only, so a checker can
never mask the staleness another gate exists to catch.

## Tests

Ship a new rule with two tests: one that fires it on a deliberate violation,
one that stays quiet on the clean case. A rule with only the first test can be
a rule that reports everything.

Ship a fix with a test that fails against the unmodified code. Write the
test, watch it fail, then fix. A test that passes before the change tested
nothing.

Test the contract, not the implementation: the finding a rule reports, its
exit status, and the text a reader acts on. Say why the case matters in a
comment when it is not obvious.

Never lower the coverage floor to make a run green. Raise it when a tier
lands. [`SPEC.md`](SPEC.md#74-testing) holds how the suite is organised and
what it covers.

## Commits

**Keep it short.** One idea per commit, and a message a reader takes in at a
glance. If a change will not fit one idea, split it.

**Subject**, always. Under 60 characters, imperative, no trailing period,
completing *"If applied, this commit will …"*. Say what the change does. Use no
category label: `fix(cli):`, `bugfix:` and `[docs]` each name a class of change
rather than the change itself, which the diff already shows. The gate fails on
one, so amend before you push.

**Body**, rarely. Most commits need none. Add one only when the subject
leaves a question a reader would otherwise have to open the diff to answer,
and then answer that question. A sentence or two does it. Wrap it at 72
columns.

Leave out how the work was scheduled, how you tested it, and what led you to
it, and stop once the question is answered. A second paragraph usually means
the message has turned into a report of the session. The reason a rule
exists belongs beside the rule in [`SPEC.md`](SPEC.md), and the
investigation that found it belongs in the pull request.

```
Reject a manifest that names a file the rework deleted
```

```
Report a skip when the tool a rule shells out to is absent

A run that verified nothing must never read as a run that verified
everything, and a machine without goreleaser is the common case.
```

Keep the `Co-Authored-By:` trailer when an agent wrote the change. Drop any
trailer linking to the agent's session or transcript. Such a link is private to
whoever ran it and dead to everyone else, and it is the one part of a commit
message that cannot be fixed after publication.

## Docs

A user-visible change lands in exactly one document. Every fact lives in one
place, and the others link to it.

| The change | Where it goes |
|---|---|
| A new rule, or a rule's behaviour | [`SPEC.md`](SPEC.md) as a numbered requirement, and the rule's own `why` string |
| A new flag or subcommand | [`MANUAL.md`](MANUAL.md) |
| A new configuration key | [`MANUAL.md`](MANUAL.md), in the table for its linter |
| A new error a user can hit | [`MANUAL.md`](MANUAL.md), under Diagnostics |
| A new prerequisite | [`INSTALL.md`](INSTALL.md) |
| A change to what the project is for | [`README.md`](README.md) |
| A convention a contributor has to follow | this file |

The rule's `why` string is documentation rather than a comment, and `--explain`
prints it. It is what a reader meets when they want to know whether a rule is
right, rather than only how to silence it.

## Publishing to npm

cs-lint is a Go binary. A project with no Go toolchain has no file in which to
pin it where a reviewer sees the version change. For a JavaScript project that
file is `package.json`.

So every release is published to npm as well. Five packages go out: four carry
the binary, one per platform goreleaser builds. The fifth is the wrapper, which
depends on all four optionally and picks the right one at run time.

Only the wrapper is written by hand, under `npm/cs-lint/`. The other four are
generated from goreleaser's output, and nothing under `npm/dist/` is committed.

```bash
make npm-snapshot          # build every target, package it, and show what would publish
make npm-build             # package whatever dist/ already holds
make npm-publish           # platform packages first, then the wrapper
```

The ordering in `npm/publish.sh` is not a preference. The wrapper is the name
people install, and it depends on packages that must already exist when it
does. Publish it first and every install between the two commands resolves a
binary the registry does not have.

Two variables belong to this packaging rather than to the tool, which is why
[`MANUAL.md`](MANUAL.md) does not carry them:

| Variable | Effect |
|---|---|
| `CS_LINT_BINARY` | Overrides the binary the npm wrapper runs, so the packaging can be exercised against a local build rather than a published one. |
| `CS_LINT_NPM_VERSION` | Names the version the generated packages carry. A tagged release supplies its own; a snapshot has none that npm will accept. |
| `CS_LINT_NPM_TAG` | The channel a prerelease is published to, `next` unless it says otherwise. The dev builds use `dev`. |
| `CS_LINT_REGISTRY_PORT` | The port the registry on this machine listens on, 4873 unless it says otherwise. |

### Dev builds

The `release` workflow, run by hand, publishes a build of whatever commit it
runs on to the `dev` channel. `node npm/build.mjs --dev` versions it as the
binary versions itself, which is Go's pseudo-version: the commit's timestamp
and its hash. So the package on the registry and the binary inside it answer
the same string, and no tag is cut.

A caret range never resolves to a prerelease, so `latest` does not move and
nobody gets one of these without asking:

```bash
npm install --save-dev @codesweep-ai/cs-lint@dev
```

### Trying the packages before publishing them

Installing a tarball by path skips the resolution that decides which of the
four platform packages a machine downloads, which is the part most worth
testing. `make npm-local` runs a registry on this machine instead, publishes to
it, and prints the address to browse:

```bash
make npm-local                 # build, publish, print the URL
npm/local-registry.sh stop     # stop it again
```

It replaces the version it published last time, so it can be run after every
change. It reaches no registry but the one it started: the credentials are a
throwaway token under `npm/.local-registry/`, which nothing outside that
directory reads.

Go marks a binary built from a modified tree with `+dirty`, which npm accepts
and then discards. That would publish the dirty build under the clean commit's
version, so the packaging rewrites it to `-dirty`, the way cs-sandbox does for
its image tags. A release never carries one, so the two cannot collide.

## Adding a rule

1. Decide which linter it belongs to, and give it the next identifier in that
   family.
2. Write the check in that package's rule table, with a title and a `why` that
   says what it wants and why it exists.
3. Add the requirement to [`SPEC.md`](SPEC.md), renumbering what follows.
4. Add a test that fires it, and a test that does not.
5. Run it against every sibling repository before you gate on it. A rule that
   lands red everywhere gets waived everywhere, which is worse than no rule.

Tune until every reported problem is a real one. A check that cries wolf is
worse than no check.

## Writing

Six principles do most of the work. Read them before you write a document,
and apply them when you edit one:

1. **Introduce a term where you first use it**, in the same sentence, or link
   to the page that defines it. A reader should never meet a word the docs have
   not explained.
2. **State the point first, then qualify it.** Opening with the qualifier makes
   the reader decode the sentence backwards.
3. **Give every sentence a subject and a verb.** "Two version numbers, one
   verdict, one remedy" reads as knowing rather than clear. Say what the thing
   is.
4. **A how-to is steps that work.** Put the reasons somewhere else. A reader
   working through one wants commands that run.
5. **Describe what the software does, not how it came to do it.** Leave out
   what the project used to do, what was tried and dropped, and numbers from a
   run somebody did once.
6. **Do not explain a design by contrast with a worse one.** Say what it is and
   what you get, rather than asking the reader to picture a design nobody
   proposed.

The mechanical rules are enforced rather than restated here. `cs-lint prose`
carries them, `make check` runs it over this repository, and `--explain` prints
what each one wants and the guidance behind it:

```bash
cs-lint prose --explain
```

That listing is the authority. Where this section and the linter disagree,
the linter is right. Every knob lives in [`.cs-lint.yaml`](.cs-lint.yaml),
and a check that reports noise is a check to fix rather than a report to
work around.

## AI-assisted contributions

An agent wrote most of this repository, and you are welcome to use one. The
standard is the same either way: you are responsible for what you submit.

Point your tool at [`AGENTS.md`](AGENTS.md), which routes it to the documents
that hold the conventions, and check three things before you open the pull
request:

- You understand every line, and can answer a question about it without going
  back to the tool.
- You ran `make check` and it passed.
- You cut what the tool added to fill space. A model pads a commit body to the
  shape it was shown, and comments that restate the code around them. Both read
  as noise to a maintainer, and both are yours to remove.

Keep the `Co-Authored-By:` trailer, which is how the work is disclosed. An
unattended agent must not open pull requests or comment on this repository.
