package refs

import (
	"fmt"
	"strings"
)

// Review is one judgement pass: what a pattern cannot decide.
type Review struct {
	ID       string
	Title    string
	Evidence []string
	Ask      string
}

// Reviews is the pack, in the order to run it. Each follows the route a reader
// takes through the documents, which is what the reference rules resolve one
// link at a time and no pattern can judge as a whole.
var Reviews = []Review{{
	ID:       "REV-R1",
	Title:    "The first ten minutes",
	Evidence: []string{"cat README.md", "cat INSTALL.md"},
	Ask: `Take the position of somebody who has just found this project and has decided to
try it. Read README.md and then INSTALL.md in that order, and stop at the first sentence
you cannot act on.

Report, in reading order, every place where the page asks you to know something it never
told you: a term used before it is introduced, a file you were never given, a command
whose output you cannot predict, a decision you are asked to make with no basis, a step
whose success you cannot tell from its failure.

For each one quote the line and give the sentence that would fix it.`,
}, {
	ID:       "REV-R2",
	Title:    "The concepts the software actually has",
	Evidence: []string{"cat README.md", "cat SPEC.md"},
	Ask: `List the concepts a user has to hold to use this software: the nouns its commands
act on, the relationships between them, and the rules a user cannot infer. Then check each
one against the documents.

Report the concepts the documents name but never explain, the ones they explain twice in
different words, and the ones the software has that they never name at all. Say which
document each belongs in.`,
}, {
	ID:    "REV-R3",
	Title: "The agent's first minutes",
	Evidence: []string{"cat AGENTS.md", "cat CONTRIBUTING.md",
		"sed -n '/Notes for agents/,/^## /p' MANUAL.md"},
	Ask: `Take the position of a coding agent that has just been given this repository and a
task. You enter through AGENTS.md, not the README.

Answer four questions using only what the repository says, and quote the line that
answered each: what am I allowed to change, what do I run before committing, where do I
put something I found but am not fixing, and which commands will block waiting for a
human. Where the answer is not in the repository, say so and name the document it belongs
in.

Then test the claims addressed to you. For every command the manual calls
non-interactive, find its implementation and confirm it prompts for nothing and waits for
nothing. Report each one that does.`,
}}

// RenderReviews returns the pack a model reads.
func (l *Linter) RenderReviews() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Reference review pack for %s\n\n", l.set.Tool())
	b.WriteString("Each section below is one review. Run them one at a time, gather the\n")
	b.WriteString("evidence named, and report findings with a document and a line.\n\n")
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
