# Contributing to cs-lint

Bug reports and pull requests are welcome. These rules apply to humans and
coding agents alike. If you are an agent working in this repository, read this
file before you change anything and follow it.

For a security issue, use GitHub's private vulnerability reporting on this
repository's Security tab, rather than opening a public issue.

## How a change gets in

File a bug or an idea as a GitHub issue on this repository. For a fix that
stands on its own, a pull request on its own is enough. For anything that adds
a rule, changes a report or moves a boundary, open an issue first, so the
design gets settled before you write it.

1. Fork the repository, and create a branch off `main`.
2. Make the change, with its test.
3. Run `make check`, which is the same gate CI runs.
4. Open a pull request against `main`, and say what the change does and why.

Review asks four questions. Does the change hold the invariants below? Does a
test fail without it? Does every user-visible change land in exactly one
document? Does the history read the way this file describes? Expect comments
rather than silence, and expect a small change to move quickly.

By opening a pull request you agree that your contribution ships under the
[Apache 2.0 licence](LICENSE) this project is released under.

## Before you push

One command:

```bash
make check
```

It runs the formatter check, `go vet`, the Go linters, the whole-program dead
code check, the unit suite and the coverage floor. It then runs all three of
this project's own linters over this repository. It is the same gate CI runs, so
a green run here is a green run there.

Two of those need tools that are not in the Go distribution:

```bash
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.12.2
go install golang.org/x/tools/cmd/deadcode@latest
```

## What a change must not break

**A rule never guesses.** Every check compares a document against something
that cannot lie: the tool's own help output, the source, the build file, the
git history, or the command re-run now. A check that reports a maybe trains
everyone to read past the whole linter. Review enforces this, and so does the
rule that anything needing judgement goes in a review pack instead.

**The tool carries no project knowledge.** No repository name, no glossary, no
path belongs in a rule. Everything that differs between repositories lives in
`.cs-lint.yaml`, which is what lets a fix to a check reach every project
without carrying one project's exceptions into the next. Review enforces this.

**Every exception is reviewable.** A waiver is a rule identifier and the reason
it was traded away, and the reason is printed with the finding. A waiver with
no reason is a rule deleted in private. Enforced by the configuration schema,
where the reason is the value rather than an optional comment.

**A check that could not run reports a skip.** Never a pass. A run that
verified nothing must never read as a run that verified everything. Enforced by
the tests, which assert a skip where a tool is absent.

**One broken check does not hide the rest.** A rule that fails while running is
reported as that rule failing, and the other sixty still run. Enforced by a
test that panics inside a rule and asserts the run survives.

**The tool writes nothing.** Every subcommand is read-only. A checker that
writes can mask the staleness another gate exists to catch. Review enforces
this.

## Tests are part of the change

A new rule ships with a test that fires it on a deliberate violation and stays
quiet on the clean case. Both halves matter: a rule with only the first test
can be a rule that reports everything.

A fix ships with a test that fails against the unmodified code. Write the test,
watch it fail, then fix. A test that passes before the change tested nothing.

Coverage is measured on every `make test` and gated at a floor by
`make coverage-check`. Raise the floor when a tier lands; never lower it to
make a run green.

## Commits

Keep one idea per commit. If a change will not fit that shape, it is doing more
than one thing, so split it.

**Subject**, always. Under 60 characters, imperative, no trailing period,
completing *"If applied, this commit will …"*. Say what the change does.

**Body**, only when the subject leaves a real question a reader would otherwise
have to open the diff to answer. Write the answer in plain English, in whole
sentences, addressed to somebody who was not there. Wrap it at 72 columns. Most
commits need no body at all.

Say what the change does and what constrained it. Leave out how the work was
scheduled, how it was tested, and what prompted it. The reason a rule exists
belongs beside the rule in [`SPEC.md`](SPEC.md), and the investigation that
found it belongs in the pull request.

Where a body carries more than one independent point, one line each reads
better than a paragraph. Never reach for another point to fill the shape. A
line that restates the subject in different words is worse than no body, and a
body written to a length is the commonest way a message stops being read.

```
Reject a manifest that names a file the rework deleted
```

```
Report a skip when the tool a rule shells out to is absent

A run that verified nothing must never read as a run that verified
everything, and a machine without goreleaser is the common case.
```

```
Sort the findings by rule, then by path

- Map order made two runs of one repository disagree.
- The report is diffed in CI, so order is data.
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

Six principles carry the voice. Read them before you write a document, and
apply them when you edit one:

1. **Introduce a term where you first use it**, in the same sentence, or link
   to the page that defines it. A reader should never meet a word the docs have
   not explained.
2. **State the point first, then qualify it.** Opening with the qualifier makes
   the reader decode the sentence backwards.
3. **Give every sentence a subject and a verb.** "Two version numbers, one
   verdict, one remedy" reads as knowing rather than clear. Say what the thing
   is.
4. **A walkthrough is steps that work.** Put the reasons somewhere else. A
   reader working through one wants commands that run.
5. **Describe what the software does, not how it came to do it.** Leave out
   what the project used to do, what was tried and dropped, and numbers from a
   run somebody did once.
6. **Do not explain a design by contrast with a worse one.** Say what it is and
   what you get, rather than asking the reader to picture a design nobody
   proposed.

The mechanical rules are enforced rather than restated here. `cs-lint docs`
carries them, `make check` runs it over this repository, and `--explain` prints
what each one wants and the guidance behind it:

```bash
cs-lint docs --explain
```

That listing is the authority. Where this section and the linter disagree, the
linter is right and this section is a bug. Every knob lives in
[`.cs-lint.yaml`](.cs-lint.yaml), and a check that reports noise is a check to
fix rather than a report to work around.

**What not to change.** This project's voice is a strength: concrete,
opinionated, free of marketing padding. These rules are about mechanics. Where
one of them fights the voice, the voice wins, and the exception is worth a
sentence in the pull request.

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
