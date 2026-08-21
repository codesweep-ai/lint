package oss

import (
	"fmt"
	"strings"
)

// Review is one judgement pass: what a pattern cannot decide.
//
// Whether a paragraph reveals an unreleased plan is not a regex, so the
// judgement half of this linter is a set of questions rather than a set of
// checks. Each one names the evidence to gather first, so a review starts from
// the same material every time it runs.
type Review struct {
	ID       string
	Title    string
	Evidence []string
	Ask      string
}

// Reviews is the pack, in the order to run it.
var Reviews = []Review{{
	ID:       "REV-01",
	Title:    "Material that should not become public",
	Evidence: []string{"git ls-files", "git log --format='%h %s%n%b'"},
	Ask: `Read every tracked file and the whole commit history of this repository. You are
looking for material that is fine internally and wrong in public: a customer or employer
named without their agreement, an unreleased plan or roadmap, an internal system,
dashboard or ticket URL, a machine on a private network, a colleague's name or handle,
revenue or headcount, a screenshot showing a private screen, a document written for an
audience inside one company.

The mechanical scans have already covered home directories, mail addresses and
credentials. Do not repeat them. Report only what needs a person to recognise it.

For each finding give the file and line, quote it, and say what it reveals.`,
}, {
	ID:       "REV-02",
	Title:    "A stranger's first clone",
	Evidence: []string{"cat README.md INSTALL.md", "cat Makefile", "ls .github/workflows"},
	Ask: `Take the position of somebody who has just cloned this repository and has none of
the author's machine. Work through INSTALL.md and the build instructions literally, and
report every step that would fail or that assumes something undeclared: a tool that is
never named, a sibling checkout, an environment variable with no default, a service that
must already be running, an operating system the docs never mention, a version floor
stated nowhere.

Say for each one whether it is a documentation fix or a code fix.`,
}, {
	ID:       "REV-03",
	Title:    "The first minute",
	Evidence: []string{"head -60 README.md"},
	Ask: `Read only the first screen of README.md, as a stranger who arrived from a search
result would. Answer four questions and quote the line that answered each: what is this,
who is it for, what problem does it solve, and what is the shortest path to seeing it
work. Where the README does not answer one, say so and propose the sentence that would.

Then judge the claim in the tagline. Is it true of what this repository actually does,
and would somebody who used the software agree with it?`,
}, {
	ID:       "REV-04",
	Title:    "What the software does to the person running it",
	Evidence: []string{"cat SPEC.md", "cat MANUAL.md"},
	Ask: `Threat-model the published artifact, not the repository. What does this software
touch on the machine that runs it: credentials, network, the filesystem outside its own
directory, other processes, a container engine, a kernel feature. For each one, say
whether the documentation tells the user before they run it.

Report anything that would surprise a careful user, anything that runs with more
privilege than it explains, and anything that sends data anywhere. A public repository is
read by people looking for exactly this.`,
}, {
	ID:       "REV-05",
	Title:    "Attribution and borrowed work",
	Evidence: []string{"git ls-files", "cat go.mod package.json 2>/dev/null"},
	Ask: `Find everything in this repository that came from somewhere else: vendored
source, a file copied from another project, an algorithm transcribed from a paper or a
blog post, generated code, a fixture captured from a third-party service, an image or a
font.

For each, say where it came from, what licence it carries, and whether this repository's
licence and its attribution satisfy that licence. Report anything whose provenance you
cannot establish, because that is the case that has to be resolved before publication
rather than after.`,
}, {
	ID:       "REV-06",
	Title:    "Consistency with the sibling projects",
	Evidence: []string{"cat README.md CONTRIBUTING.md AGENTS.md"},
	Ask: `Compare this repository against the conventions its siblings follow: an H1 that
is the repository name, one bold sentence of claim, a badge block, an ASCII diagram where
the shape is not obvious, a Quickstart, a Docs link list, and a License line.
CONTRIBUTING carries the commit rules, the writing rules and the doc map. AGENTS.md
routes and holds no knowledge of its own.

Report every place this repository reads as a different project rather than a member of
the same family, and propose the specific edit.`,
}, {
	ID:       "REV-07",
	Title:    "Claims the code does not support",
	Evidence: []string{"cat README.md MANUAL.md"},
	Ask: `Every sentence in the documentation that states a fact about the software is a
claim. Check the load-bearing ones against the source: the performance numbers, the
platform support, the guarantees, the "never" and "always" statements, and the list of
what is supported.

Report each claim you could not ground in the code, and say what the code actually does.
A claim that survives publication and turns out to be false is the expensive kind.`,
}, {
	ID:       "REV-08",
	Title:    "The history a reader scrolls",
	Evidence: []string{"git log --format='%h %s' | head -80", "git log --format='%B' | head -200"},
	Ask: `Read the commit history as a stranger evaluating whether to depend on this
project. Report subjects that say nothing ("fixes", "wip", "updates"), bodies that
narrate a debugging session rather than describe a design, messages that reference an
internal ticket or a conversation the reader cannot see, and any message that would
embarrass someone.

Say for each whether it is worth rewriting before publication, given that rewriting is
possible now and impossible afterwards.`,
}, {
	ID:       "REV-09",
	Title:    "The lines that did not earn their place",
	Evidence: []string{"git log -60 --format='%s%n%b%n---'", "cat CONTRIBUTING.md"},
	Ask: `Read the last sixty commit bodies against the commit convention in CONTRIBUTING.md.
You are looking for the line that exists to fill a shape rather than to say something:
one that restates the subject in other words, one that opens with "Also" and adds a
detail nobody needed, and one that describes the diff rather than the design.

Then look at the distribution rather than at any single message. Count how many bodies
carry no lines, one, two, three and more. A count that piles up on one number is the
tell: the convention named that number, or an example in the document showed it, and
both read as a target rather than as a limit.

Report the padded lines with their commit, and say what the document should say instead.
Where the document names a number or prints an example at its own ceiling, say so. That
is the cause, and the messages are the symptom.`,
}}

// RenderReviews returns the pack, which is a document rather than a run: what
// a reader does with each finding is theirs to decide.
func (l *Linter) RenderReviews() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Review pack for %s\n\n", l.project())
	b.WriteString("Each section below is one review. Run them one at a time, gather the\n")
	b.WriteString("evidence named, and report findings with a file and a line.\n\n")
	b.WriteString("The mechanical rules have already run. Nothing here repeats them.\n\n")
	for _, r := range Reviews {
		fmt.Fprintf(&b, "## %s: %s\n\n", r.ID, r.Title)
		b.WriteString("Evidence to gather first:\n\n```bash\n")
		for _, e := range r.Evidence {
			b.WriteString(e + "\n")
		}
		b.WriteString("```\n\n")
		b.WriteString(strings.TrimSpace(r.Ask) + "\n\n")
	}
	return b.String()
}
