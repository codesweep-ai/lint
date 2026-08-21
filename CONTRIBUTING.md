# Contributing to cs-lint

Bug reports and pull requests are welcome. For a security issue, use GitHub's
private vulnerability reporting on this repository's Security tab, rather than
opening a public issue.

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

## What this project will not trade away

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

**Body**, only when the subject leaves a real question. Use bullets, one line
each, under 60 characters, describing the design: the shape the change takes,
or the constraint that ruled out the obvious alternative. Do not describe the
diff, and do not describe how you arrived at it.

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

These rules come from
[Google's developer documentation style guide](https://developers.google.com/style),
from the
[Red Hat supplementary style guide](https://redhat-documentation.github.io/supplementary-style-guide/),
and from repeated review of what actually confuses readers. `cs-lint docs`
checks the mechanical half, and running it against this repository is part of
`make check`.

1. **Introduce a term where you first use it**, in the same sentence, or link
   to the page that defines it. A reader should never meet a word the docs have
   not explained.
2. **Give every sentence a subject and a verb.** "Two version numbers, one
   verdict, one remedy" reads as knowing rather than clear. Say what the thing
   is.
3. **State the point first, then qualify it.** Opening with the qualifier makes
   the reader decode the sentence backwards.
4. **Keep sentences under 30 words**, and to one idea each.
5. **No em-dash.** The aside one introduces is a full stop, a comma, or a cut.
   It is also the first punctuation a model reaches for, so a page full of them
   reads as unedited whoever wrote it.
6. **Address the reader as "you"**, and use the imperative for steps.
7. **Keep the evidence out of the instructions.** A war story explains a
   decision. Put it in an explanation section, not in the middle of a task.
8. **Make every example runnable as written.** If a step invokes a script, show
   the script first. A reader should never meet a file they were not given.
9. **Do not comment on your own writing.** Delete the frame and keep the
   sentence: "it is worth stating plainly", "put simply", "the point is".
10. **Do not explain a design by contrast with a worse one.** Say what it is
    and what you get.
11. **Leave out what does not matter.** Every fact you print is one the reader
    has to decide whether to act on.
12. **A walkthrough is steps that work.** Put the reasons somewhere else.
13. **An ordered procedure is a numbered list, not a sentence.**
14. **Describe what the software does, not how it came to do it.** Leave out
    what the project used to do, what was tried and dropped, and numbers from a
    run someone did once.
15. **Do not make the reader hold two halves of a sentence apart.** Name the
    subject in each clause.

**What not to change.** A project's voice is usually a strength: concrete,
opinionated, free of marketing padding. These rules are about mechanics. Where
one of them fights the voice, the voice wins, and the exception is worth a
sentence in the pull request.
