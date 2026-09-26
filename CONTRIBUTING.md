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
3. Run `make ci`, which is every gate CI runs.
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

A run that passes on a clean tree also records its commit as a local build,
so a sibling project can pin it before it is pushed.
`scripts/record-build.sh` files it, with its module zip and its npm
packages, in the build store of the repository's owner,
`~/.local/share/cs-builds/<owner>/`, and says so. A run over uncommitted
changes records nothing.

No linter needs installing. The ones it shells out to are pinned Go tools,
built from the module cache the first time you run them: `golangci-lint`,
`deadcode`, `actionlint` and `cs-ledger`. `make repin` moves each `cs-` pin to
the newer of its project's last CI build and its newest local one, which a
clean `make ci` records. It names each pin taken from a local build, and
leaves one whose project has neither. `make repin LOCAL=0` takes CI builds
only. `make versions` prints the version of each.

Moving a pin is an edit to `go.mod`, or to `go.golangci.mod` for
`golangci-lint`. A linter release reaches you when you ask for it, not on an
unrelated pull request.

`goreleaser` is the one program still expected on the PATH. `make ci`
validates the release manifest with it, and `make build` falls back to
`go build` where it is absent. `make install` also packs the npm packages with it,
node and npm, and skips that step where any of them is absent.

This repository keeps a **ledger** of open issues in `ledger/`. Read
[`ledger/AGENTS.md`](ledger/AGENTS.md) before you start work, and follow it as
you go. A commit that touches `ledger/` needs `cs-ledger render && cs-ledger
check` to pass first, and `make ledger` runs the check half.

A push to main that changes only `ledger/` builds nothing and publishes nothing
to npm or as images. `ci` does not run for it: the `ledger` workflow runs
`make ledger`, `make prose`, `make refs` and `make oss` instead, and the site
republishes the ledger's page when it finishes. Such a commit is never a build
a sibling pins.

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
completing *"If applied, this commit will …"*. Say what the change does in
plain English. The test: would this subject make sense to someone who has not
read the diff and does not know this codebase? Use no category label:
`fix(cli):`, `bugfix:` and `[docs]` each name a class of change rather than
the change itself, which the diff already shows. The gate fails on one, so
amend before you push.

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
Reject a manifest that names a deleted file
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

Every release also goes to npm as five packages: four carry the binary, one per
platform goreleaser builds, and the wrapper picks the right one at run time.
Only the wrapper is written by hand, under `npm/lint/`. The other four are
generated from goreleaser's output, and nothing under `npm/dist/` is committed.

```bash
make npm-snapshot   # build every target, package it, and show what would publish
make npm-build      # package whatever dist/ already holds
make npm-pack       # package a dev build into cs-npmrevs's data directory
make npm-local      # the same, then serve it, and print how to install it
make npm-publish    # platform packages first, then the wrapper
```

Keep that order in `npm/publish.sh`. The wrapper depends on packages that must
already exist when it is published. Publish it first, and every install between
the two commands resolves a binary the registry does not have.

Running `npm/publish.sh` again is safe. It skips each package the registry
already has from this commit, and stops on one it has from another commit.

`make npm-local` is how to try a package before publishing it. It packs the five
packages into cs-npmrevs's default data directory and serves them with
[cs-npmrevs](https://github.com/codesweep-ai/npmrevs), which makes every
revision of an npm package installable without publishing it. It runs the
cs-npmrevs that `go.mod` pins, and takes every other package from npmjs.com. It
prints the install command, with the exact version it built. Run it after every
change: a rebuild of the same commit replaces the last run's tarballs.
`npm/local-registry.sh stop` stops the server. Every project's build shares
that directory, so `make npm-pack` alone leaves a build for a later one to
install through cs-npmrevs on port 4875. `make install` runs it too.

These variables belong to the packaging rather than to the tool, which is why
[`MANUAL.md`](MANUAL.md) does not carry them:

| Variable | Effect |
|---|---|
| `CS_LINT_BINARY` | The binary the npm wrapper runs, so the packaging can be tried against a local build. |
| `CS_LINT_NPM_VERSION` | The version the generated packages carry. A tagged release supplies its own. |
| `CS_LINT_NPM_TAG` | The channel a prerelease is published to, `next` unless it says otherwise. |
| `CS_NPMREVS_PORT` | The port `make npm-local` serves on, 4875 unless it says otherwise. |
| `CS_NPMREVS_IMAGES`, `CS_NPMREVS_SCOPE` | The images registry and the scope `make npm-local` serves beside the data directory, `ghcr.io` and `@codesweep-ai` unless they say otherwise. |
| `CS_NPMREVS_DATA` | The data directory `make npm-pack` and `make npm-local` pack into, cs-npmrevs's own default unless it says otherwise. |
| `NPMREVS` | The command `npm/local-registry.sh` and `npm/publish-images.sh` run as cs-npmrevs. `npm/local-registry.sh`, `make images-snapshot` and the workflow use the pinned one, and `npm/publish-images.sh` run by hand uses `cs-npmrevs` from the PATH. |
| `REGISTRY` | The registry `npm/publish-images.sh` publishes to, `ghcr.io` unless it says otherwise. |

### Dev builds

The `npm` workflow publishes every commit on main that passes `ci` to the `dev`
channel, cutting no tag and making no release. It runs when `ci` finishes, and
builds the commit `ci` tested. It skips that commit once main's head changes
more than `ledger/` after it, and the head publishes when its own `ci` passes.
Every publish also writes an `npm` commit status to its commit, a failed one
included. It stores no credential:
each package names the workflow as a trusted publisher. To publish one commit by
hand, or to see what a publish would send without sending it, run it from the
Actions tab or with `gh workflow run npm.yml`. A dispatched publish waits for
`ci` to pass on that commit, and a dry run does not wait. After a push that
changes only the ledger, the head has no `ci` run, so a dispatch there
publishes nothing.

Such a build takes its version from the binary, which is the commit's timestamp
and its hash. A caret range never resolves to a prerelease, so one reaches
nobody who has not asked:

```bash
npm install --save-dev @codesweep-ai/lint@dev
```

In a fork, or a copy under another owner, the images are still published but
nothing goes to npm, because the packages there take that owner's scope.
That owner publishes them by running the `npm` workflow by hand, once each
package names it as a trusted publisher. A trusted publisher can only be added
to a package that exists, so the first publish runs `npm/publish.sh` from a
machine logged in to npm.

### Images of the packages

The `publish images` workflow pushes each commit on main that passes `ci` as
five images, one per package, such as `ghcr.io/codesweep-ai/npm/lint:<version>`.
It starts when `ci` finishes, and pull requests get no image. The `prune-images`
workflow keeps the newest 20 versions of each package. Those images let a team
of AI coding agents install builds that are not yet meant for people:
[cs-npmrevs](https://github.com/codesweep-ai/npmrevs) serves them to npm, or
copies them into a directory.

Each run posts a `publish images` commit status on the commit it published: a
success once the wrapper is pushed, a failure otherwise. GitHub lists the run
under main's head when `ci` finished, which can be a later commit. The CI status
file reads the registry instead, and lists a commit as built, one a sibling can
pin, once its wrapper's version is there.

`npm/publish-images.sh` builds each image with cs-npmrevs and pushes it with
podman. The workflow is what runs it, and `make images-snapshot` builds the
images of whatever `npm/dist/` holds, pushing nothing. The script is shared with
npmrevs, ledger and ui, so change all four together.

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
- You ran `make ci` and it passed.
- You cut what the tool added to fill space. A model pads a commit body to the
  shape it was shown, and comments that restate the code around them. Both read
  as noise to a maintainer, and both are yours to remove.

Keep the `Co-Authored-By:` trailer, which is how the work is disclosed. An
unattended agent must not open pull requests or comment on this repository.
