# The cs-lint specification

cs-lint is a command-line tool that checks a repository against four sets of
rules. Two of them cover how its documents are written and whether everything
those documents point at is still there. The other two cover whether the
interface they describe is the real one, and what a published repository owes a
reader. It reads the
repository, the git history, and the tool the repository ships. It reports what
is wrong with a rule identifier and an address.

This document states what the tool guarantees and how it is built. Read the
vocabulary first: the rest of the document stops explaining itself after it.

## 1. Purpose

Every artifact in a repository is executed by something. Code is compiled,
tests are run, schemas are validated, and a release is built. Documentation is
read, and reading is the one form of verification that cannot fail. What a
repository owes a reader before publication has the same problem: nothing runs
it, so it is true on the day somebody checks and drifts afterwards.

### 1.1 Goals

1. Report only what a machine can decide without guessing.
2. Give every finding a rule identifier, an address, and the text that proves it.
3. Carry no project knowledge in the tool, so a fix to a check reaches every
   repository that uses it.
4. Make every exception reviewable: a waiver names the rule and states why.
5. Run from one binary with no runtime to install.

### 1.2 Non-goals

1. Judgement. Whether a paragraph reveals an unreleased plan is not a pattern,
   and the tool prints a review pack for that half rather than pretending to
   decide it.
2. Rewriting. cs-lint reports; it does not edit the repository.
3. cs-lint drives no other tool to act on what it reports. The review packs are
   documents on standard output, and what a reader does with one is theirs to
   decide.
4. It is not a general style engine. The declined-terms table is deliberately short.
5. Language-specific static analysis is the language's own tool, and one readiness
   rule checks that it is wired in.

## 2. Vocabulary

| Term | Meaning |
|---|---|
| **linter** | One of the four rule sets: `prose`, `refs`, `surface`, `oss`. |
| **rule** | One check, with an identifier such as `OSS-102`, a severity, and a stated reason. |
| **finding** | One reported problem: a rule, a severity, a message, and where to look. |
| **gate** | A command that fails a build when a rule reports an error. |
| **waiver** | A rule turned off for one repository, with the reason recorded beside it. |
| **leak** | Material in the tree or the history that was never meant to leave the machine it was written on. |
| **router** | `AGENTS.md`, the file an agent harness discovers by name and reads first. |
| **help tree** | Every verb a tool carries, walked from its own help output. |
| **citation** | A reference to a numbered section, written `SPEC.md §7.2` or with a bare `§`. |
| **sample** | A recorded command and the output a document claims it prints. |
| **elision** | A `…` in a sample, standing for whatever the command prints there. |
| **epigram** | A short verbless sentence, such as "Two version numbers, one verdict, one remedy." |
| **family** | The sibling repositories that share these conventions. |

## 3. Interfaces

### 3.1 The command line

```
cs-lint prose    [--stats] [--list] [--explain]
cs-lint refs     [--review] [--list] [--explain]
cs-lint surface  [--run] [--review] [--list] [--explain]
cs-lint oss      [--online] [--review] [--list] [--explain]
cs-lint manual
cs-lint version
```

`--root` and `--verbose` apply to every subcommand.

### 3.2 The tuning file

The tuning file is `.cs-lint.yaml` at the repository root. It holds two
sections: `docs`, which carries the document set and one block per
documentation linter, and `oss`. A repository with nothing to tune may leave
the file out.

### 3.3 What the tool reads

The tracked files, the git history, `.cs-lint.yaml`, `.leakterms` if present,
and the binary the repository builds. Nothing else.

## 4. The functional specification

### 4.1 Reporting

**R1.** Every finding **MUST** carry a rule identifier, a severity, and a
message.

**R2.** A finding about a file **MUST** carry an address of the form
`path:line` wherever a line can be determined. *An address is what turns a
report into an edit.*

**R3.** An error **MUST** fail the run. A warning **MUST** print and pass.
*A warning flags a judgement call rather than broken data, and a gate that
fails on judgement gets switched off.*

**R4.** A check that could not run **MUST** report a skip rather than a pass.
*A run that verified nothing must never read as a run that verified
everything.*

**R5.** A skip **MUST** be counted in the summary whether or not it is printed.

**R6.** A rule that fails while running **MUST** be reported as that rule
failing, and the remaining rules **MUST** still run. *One broken check must not
hide the findings of the other sixty.*

### 4.2 Exit status

**R7.** The process **MUST** exit 0 when no rule reported an error.

**R8.** The process **MUST** exit 1 when any rule reported an error.

**R9.** The process **MUST** exit 2 when it could not run at all: an unreadable
tuning file, a waiver naming no rule, an unknown flag, an unknown subcommand. *A gate needs to tell "the
repository has a problem" from "the linter is broken", because the first is a
finding and the second is a broken build.*

### 4.3 Configuration

**R10.** A repository with no `.cs-lint.yaml` **MUST** run with the documented
defaults.

**R11.** A key the schema does not define **MUST** fail the run. *A knob that
silently does nothing is the failure this format is most prone to.*

**R12.** A section the file does not mention **MUST** keep its default.

**R13.** Every waiver entry **MUST** carry a reason, and that reason **MUST**
be printed with the finding it waives. *A waiver nobody can review is a rule
deleted in private.*

**R14.** A waiver **MUST** downgrade an error to a skip, and **MUST NOT**
remove it from the report.

**R15.** A waiver naming a rule the linter it sits under does not carry **MUST**
fail the run. *An identifier that matches nothing switches a rule off and says
nothing about it, which is exactly what requiring a reason exists to prevent.*

**R16.** A waiver naming a rule identifier from before the split **MUST** report
the identifier that replaced it, and the section that now waives it.

**R17.** A tuning file written to the schema from before the split **MUST** fail
the run with the new location named. *Both shapes fail the strict decode
anyway, in the parser's own words. Naming the new location is the difference
between a message that ends the work and one that starts it.*

### 4.4 The prose linter

**R18.** Code fences, tables, link definitions and raw HTML **MUST** be
excluded before any rule is applied. *They are not prose, and none of the rules
are about them.*

**R19.** The prose extraction **MUST** preserve the line count of the document,
so an offset into the result still names the line it came from.

**R20.** A document that numbers three or more requirements in bold **MUST** be
read as a spec, and the rules written for prose **MUST NOT** be applied to it.
*A spec states obligations in the passive and speaks about the project's own
artifacts.*

**R21.** A word inside quotation marks on its own line **MUST NOT** be reported
as used. *Naming a word is not using it.*

**R22.** A glossary term **MUST** be reported when a document uses it before
glossing it, defining it in a glossary table, or linking to the page that
defines it.

**R23.** An em-dash **MUST** be reported wherever it appears. *The aside it
introduces is a full stop, a comma, or a cut, and it is the first punctuation a
model reaches for.*

**R24.** A sentence of more than thirty words **MUST** be reported.

**R25.** A sentence of three to twelve words carrying no verb **MUST** be
reported. *Above that length a sentence tripping the verb check is almost
always a real sentence with a verb the list does not carry.*

**R26.** A rule that follows published guidance **MUST** name the guide in its
explanation.

**R27.** A section of negatives in a README **MUST** be reported. *A reader
arrives to find out what the software does. Non-goals and hard limits belong in
this document, where stating them is the job, so the rule reads only the
README.*

**R28.** A number stated immediately before a declared countable **MUST** be
reported, and an empty list **MUST** disable the check. The check **MUST** read
prose alone: a number a recorded sample, a table, inline code or raw HTML holds
**MUST NOT** be read as a claim. *A count written into a sentence is right the
day it is written and wrong by the next commit, with nothing to fail when it
drifts. What a command printed is the sample check's to hold, not this one's,
and a number quoted as text is being shown rather than asserted.*

### 4.5 The readiness linter

**R29.** Every tracked file **MUST** be scanned, not a chosen subset. *Leaks
turn up in a fixture, in a golden derived from it, in a rendered page derived
from the golden, and in a script with a hard-coded path.*

**R30.** A rule whose verdict comes from reading tracked files **MUST** report
a skip where there are none to read. *A directory git cannot answer for, a
repository with nothing committed, and one tracking only binary assets all
leave such a rule inspecting zero files, and silence there is indistinguishable
from a clean scan.*

**R31.** A tracked file that cannot be read as text **MUST** be reported unless
its extension is a declared binary asset. *A file nobody can inspect must never
be reported as clean.*

**R32.** A leak pattern **MUST** match a class rather than a name. *A committed
list of private terms publishes exactly what you consider private.*

**R33.** The name of the person running the check **MUST** be taken from the
environment and **MUST NOT** be written into the tool or the tuning file.

**R34.** A home directory naming a person **MUST** be reported unless the name
is a declared placeholder, matched as a whole name. *The allowance is matched as a whole name, so
`/home/user` is a placeholder and a longer name beginning with those letters is
somebody's login.*

**R35.** A credential-shaped string that says of itself that it is fake
**MUST NOT** be reported. *A test needs a credential-shaped string, and the only safe
one says so in itself.*

**R36.** At most one leak **MUST** be reported per file. *One report is enough
to act on, and a generated page would otherwise print thousands.*

**R37.** A path the configuration declares skipped **MUST** be skipped by the
history scan as well as by the tree scan. *A waiver says "this path is not
evidence", and honouring it in one scan and not the other leaves a repository
reporting clean on its tree and red on its history for the same declared
reason.*

**R38.** The history rules **MUST** report as warnings once the repository is
declared published. *Published history cannot be rewritten, so the rule becomes
advice.*

**R39.** A rule that asks the forge about the repository **MUST NOT** run
unless asked.

**R40.** `CONTRIBUTING.md` **MUST** state how a change reaches the project:
where a report goes, where the work starts, and how it is submitted. *A
document that states every convention and never the process reads as published
rather than open, and the contribution that never arrives leaves no trace to
notice.*

**R41.** A public channel for an ordinary report **MUST** be named, distinctly
from any tracker the repository commits to its own tree. *A ledger in the tree
is the maintainers' record. A stranger cannot file into it.*

**R42.** The terms a contribution is accepted under, and a policy for
AI-assisted contributions, **MUST** each be stated. *A contributor grants a
licence whether or not anybody says so, and a repository whose history carries
agent trailers owes its readers the policy behind them.*

**R43.** The writing rules **MUST** be cited rather than restated. *A second
copy drifts. Six sibling projects each restating one list produced six
different lists, two of which stated a threshold the linter they shipped did
not hold.*

**R44.** A table `CONTRIBUTING.md` and `SPEC.md` both carry **MUST** be
reported. *A fact lives in one document and the others link to it.*

**R45.** A commit convention **MUST NOT** be checked against a number the
convention itself publishes. *A stated maximum becomes a target: where one was
named as the rare maximum, a repository put exactly that many in 31 of 149
commits. The check reports the shape; the document describes the condition.*

**R46.** A commit body that runs past a stated word count, or past two
paragraphs, **MUST** be reported. *A convention that asks for quality and never
mentions length produces long messages: plain English, whole sentences and
writing for a reader who was not there are each satisfied by writing more. The
threshold belongs in the check rather than in `CONTRIBUTING.md`, where a stated
number becomes the length messages get written to.*

### 4.6 The reference linter

**R47.** The path check **MUST** read every tracked Markdown file, not only the
document set, and a tree it skips **MUST** be declared with a reason. The
document set **MUST** be checked whatever a skip says. *A nested README makes
the same claim the document set does, and a path it names that has moved is
wrong in the same way. A skip nobody can review is a scan deleted in private.*

**R48.** A section citation in the source **MUST** resolve, and a tree it skips
**MUST** be declared with a reason. The suite **MUST** be read, and no
`sourceSkip` **MUST** apply. A citation naming no document **MUST** be read
against the spec, or against the only document that numbers its sections, and
**MUST** be left alone where neither holds. A citation naming a document the
repository does not carry **MUST** be left alone. *A comment in a test misleads
its next reader as surely as one in production code, and a tree excluded because
its settings are another program's still cites this repository's own spec. A
rule that guesses which document was meant reports a finding nobody can act on.*

**R49.** A numbered section **MUST** be recognised as a heading or as a bold
lead-in. *A spec numbering its rules without adding a level to the table of
contents is not a spec whose citations are all stale.*

**R50.** A rule in this linter **MUST NOT** depend on the binary the
repository builds. *That is what lets a gate resolve every reference before it
builds anything, and what makes this the first of the four to answer.*

### 4.7 The interface linter

**R51.** Every claim **MUST** be compared against something that cannot lie:
the tool's help tree, the source that reads a variable, the build file, or the
command re-run now. *Nothing here guesses what a document ought to say.*

**R52.** The help tree **MUST** be walked rather than assumed, to a depth of
three verbs. *A subcommand's own subcommands are where a surface goes
undocumented.*

**R53.** A verb whose help page is identical to its parent's **MUST NOT** be
walked for children. *A tool with no per-verb help answers with the page its
parent gave, and reading those as children multiplies the tree by itself at
every level.*

**R54.** A sample **MUST NOT** be re-run unless every command in it names a
declared safe verb. *A checker that writes can mask the staleness another gate
exists to catch.*

**R55.** An elision in a recorded line **MUST** match whatever the command
prints in its place, and everything either side of it **MUST** still match.

**R56.** A variable read only by test code **MUST NOT** be reported as an
undocumented setting. *A variable only the suite reads is instrumentation
rather than a setting.*

**R57.** A variable passed to a child process **MUST NOT** be read as a
setting the tool itself reads.

**R58.** Where the tool carries a `manual` verb and the repository carries
`MANUAL.md`, what the verb prints **MUST** be what the file holds, ignoring a
trailing newline. Where the verb, the file or the binary is absent, or the verb
does not exit zero, the rule **MUST** report a skip. The finding **MUST** name
the first line that differs. *The printed copy is what a machine with no
checkout reads, so a stale binary sends the wrong answer to the reader least
able to notice, and the first differing line is where a rebuild shows its work.*

**R59.** Every rule in this linter **MUST** report a skip of its own where
there is no binary to ask. *A project that has not wired a build dependency
still runs the linter, and one collective failure would tell it nothing about
which checks it lost.*

## 5. Data model

### 5.1 The tuning file

One YAML document with two top-level keys, each optional. `docs` carries the
document set and one block per documentation linter. The set sits above the
blocks because `refs` and `surface` both read it.

```yaml
docs:
  documents: []            # the set refs and surface read the claims from
  extraDocs: []            # the standalone pages the set adds

  prose:
    skipExtra: []          # directories of fixtures or generated Markdown
    glossary: []           # terms a reader cannot infer
    lowercaseStarters: []  # words that start a sentence in lower case
    projectVerbs: []       # verbs the shared list does not carry
    countable: []          # things this repository counts for itself
    terms: {}              # pattern -> what to write instead
    termsProse: {}         # the same, for documents that are not specs

  refs:
    placeholderOK: []      # placeholder paths a block may name on purpose
    prereqOK: []           # build tools no document has to name
    markdownSkip: {}       # path prefix -> why its Markdown claims nothing here
    citationSkip: {}       # path prefix -> why a section number there is not a citation
    agentSection: ""       # the manual heading addressed to automated callers
    allow: {}              # rule id -> why it is waived

  surface:
    tool: ""               # the command name
    toolPath: ""           # the binary this checkout builds
    envPrefix: ""          # the variable prefix this tool reads
    envInternal: {}        # variable -> why it is deliberately undocumented
    safeVerbs: []          # verbs a sample check may re-run
    sampleSkip: {}         # command -> why it cannot reproduce here
    sourceSkip: {}         # path prefix -> why the source under it is not this tool's
    allow: {}              # rule id -> why it is waived

oss:
  project: ""              # the command this repository ships
  githubRepo: ""           # owner/name, when the remote is not the answer
  published: false         # true once the repository is public
  docSet: []               # the documents this repository carries
  extraDocs: []            # the standalone pages it adds
  homeAllow: []            # placeholder home names, never a real login
  emailAllow: []           # documentation mail domains
  skipPaths: {}            # path prefix -> why the scans skip it
  allow: {}                # rule id -> why it is waived
  binaryOK: []             # extensions a scan may skip as known assets
  requiredTargets: []      # task-runner targets that must exist
  expectedTargets: []      # the rest of the family's vocabulary
```

`docs.surface.tool` and `docs.surface.sourceSkip` are read by the reference
rules as well. Both describe the tool rather than either linter. The first says
which name in a build file is the tool itself, and the second says which trees
hold somebody else's code. A second copy of either would let the two halves
disagree.

### 5.2 A finding

| Field | Meaning |
|---|---|
| **Rule** | The rule identifier, such as `OSS-102`. |
| **Severity** | `error`, `warning`, or `skip`. |
| **Message** | What is wrong, in one line. |
| **Where** | The file, or the file and line, that carries it. |
| **Quote** | The text that proves it, where quoting one helps. |

## 6. Configuration resolution

The tool reads `.cs-lint.yaml` from the directory `--root` names, defaulting to
the working directory. There is no user-level or system-level file, and no
environment variable overrides a knob: a check that behaves differently on two
machines is a check nobody can act on.

## 7. Implementation

### 7.1 Packages

| Package | Role |
|---|---|
| `cmd/cs-lint` | The entry point. |
| `internal/cli` | The command tree, the report format, and the exit status. |
| `internal/config` | The tuning file and the defaults. |
| `internal/lint` | A finding, a severity, and the repository the rules read. |
| `internal/mdtext` | Markdown reduced to prose, and split into sentences and units. |
| `internal/docset` | The repository as the documentation rules read it: the document set, the blocks in it, the source, the build files, and the binary. |
| `internal/prose` | The writing rules. |
| `internal/refs` | The reference rules. |
| `internal/surface` | The interface rules. |
| `internal/oss` | The readiness rules. |
| `internal/lint/linttest` | The scratch repositories the rule tests read. |
| the module root | `MANUAL.md`, embedded so the binary carries its own reference. |

The dependency direction is one way. `cli` reads the four rule packages and the
embedded manual. Each rule package reads `config` and `lint`. `prose` also reads
`mdtext`, and `refs` and `surface` both read `docset`. `config`, `lint` and
`docset` import no rule package. No rule package imports another.

`refs` and `surface` share `docset` rather than a copy each, because both read
the same document set, the same fenced blocks and the same tracked text. Two
copies would be two chances for the halves of one split to disagree about what
this repository says.

### 7.2 Key types

`lint.Problem` is a finding as data rather than a printed line. A caller can
count findings, sort them, waive one by its rule, and render them in whichever
form the command asked for.

`lint.Repo` is the repository and the cache the rules read it through. Every
rule over a few hundred files would otherwise read the same file many times, so
everything is read once and kept.

Each rule package holds its own table, because each needs a different context to
run a check against. What they share is `lint.Guard`, which executes one check.
It turns a panic into a finding against whichever rule raised it, so one broken
check cannot hide the rest. It stamps the identifier onto findings that carry
none, and it stops any finding reporting louder than the rule it came from.

### 7.3 Regular expressions

Go's regular expressions guarantee linear-time matching and therefore carry no
lookahead, lookbehind or backreference. Where a rule needs one, the pattern
matches the shape and the surrounding condition is checked beside it. An
allowlist expressed as a negative lookahead becomes a set lookup, which is both
faster and easier to read.

### 7.4 Testing

Each rule package has a table of deliberate violations, one per check. It
asserts that the check fires on each, and stays quiet on the clean case. The
command tree is driven end to end against scratch repositories, which is what
covers the report format and the exit status. Coverage is measured on every
test run and gated at a floor.

## 8. Conformance

An implementation conforms when it satisfies R1 through R59, and when:

1. `cs-lint <linter> --explain` prints every rule it carries, with its reason.
2. Every rule that reports a finding appears in that listing.
3. A repository with no tuning file runs with the documented defaults.
4. The exit status follows R7 through R9.

## 9. Quality attributes

```
[QA-01] No guessing: every rule is mechanical and quotable
  Measured by: each rule's test asserts a clean case reports nothing
  Classification: BEHAVIORAL

[QA-02] No project knowledge in the tool: a check moves without exceptions
  Measured by: the rule packages read config and never a repository name
  Classification: STRUCTURAL

[QA-03] Reviewable exceptions: every waiver states why
  Measured by: the waiver map's value is required, and printed with the finding
  Classification: BEHAVIORAL

[QA-04] One binary, no runtime: a gate needs nothing installed first
  Measured by: the module depends on a command-line library and a YAML parser
  Classification: STRUCTURAL
```

## 10. The hard limits

**Judgement is out of reach.** No pattern decides whether a sentence reveals something private, a concept went
unexplained, or a claim is true of the software. The review packs exist because the alternative is a
linter that guesses, and a linter that guesses produces noise.

**A binary blob is opaque.** The history scan reads the text of each diff, so a
leak inside a compiled file or an editor swap file that was later deleted is
past it.

**A sample can only be re-run when it is safe to run.** A verb that writes
cannot be re-run on every gate, so a document that only ever shows writing
commands has samples nothing checks.

**The tool cannot know what a document ought to say.** It can say that a
document names a command the binary lacks. It cannot say that a document is
missing the paragraph a reader needed.

## 11. Open questions

1. **The state-path rule is shadowed.** A path into a person's own state is also a home directory. The scan reports the
   first pattern that matches a file, so `OSS-302` fires only where `OSS-301`
   does not. The rule earns its
   place in the explanation rather than in the output. Whether to report both
   is undecided.

2. **The declined-terms table has no per-term severity.** Every entry is an
   error. A term added from weaker evidence would be better as a warning, and
   nothing expresses that today.

3. **The glossary check reads the first use only.** A term introduced properly
   on first use and then used loosely twenty pages later is not reported.

4. **There is no cross-repository check.** These conventions exist so sibling
   repositories read alike, and nothing compares one against another. The
   consistency review covers it by judgement.

5. **The help tree walk is bounded at three verbs.** No repository in the
   family has gone deeper, and a tool that did would have its fourth level
   unchecked with no report saying so.
