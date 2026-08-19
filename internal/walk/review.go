package walk

import (
	"fmt"
	"path/filepath"
	"strings"
)

// shellNoise are words that read as a command in prose but are not this tool's.
var shellNoise = map[string]bool{
	"cd": true, "ls": true, "cat": true, "echo": true, "export": true,
	"sudo": true, "source": true, "curl": true, "grep": true, "sed": true,
	"awk": true, "mkdir": true, "rm": true, "cp": true, "mv": true,
	"tar": true, "install": true, "chmod": true, "chcon": true, "git": true,
	"make": true, "go": true, "npm": true, "node": true, "python3": true,
	"sh": true, "bash": true, "open": true, "less": true, "head": true,
	"tail": true, "diff": true, "find": true, "xargs": true, "jq": true,
	"ssh": true, "podman": true, "docker": true, "kubectl": true,
	"brew": true, "apt": true, "dnf": true, "systemctl": true,
}

// Row is one line of the walkthrough's checklist.
type Row struct {
	Where   string
	Kind    string // safe, tool, host, other
	Command string
	Note    string
}

// Inventory returns every command the documents tell a reader to run, in
// reading order.
//
// This is the walkthrough's checklist. A reader works down it, and so does an
// agent: each line names the document, the line, and the command, so a run can
// be recorded against a stable address rather than a paraphrase.
func (l *Linter) Inventory() []Row {
	tool := l.Tool()
	safe := map[string]bool{}
	for _, v := range l.cfg.SafeVerbs {
		safe[v] = true
	}
	var rows []Row
	for _, block := range l.Blocks() {
		if !isShell(block.Lang) {
			continue
		}
		for _, command := range block.Commands {
			words := strings.Fields(command)
			if len(words) == 0 {
				continue
			}
			head := filepath.Base(words[0])
			kind := "other"
			switch {
			case head == tool:
				kind = "tool"
				if safe[verbOf(words[1:])] {
					kind = "safe"
				}
			case shellNoise[head]:
				kind = "host"
			}
			note := ""
			for _, p := range placeholders {
				if hit := p.FindString(command); hit != "" {
					note = "needs a path you supply: " + hit
					break
				}
			}
			rows = append(rows, Row{block.Where(), kind, command, note})
		}
	}
	return rows
}

// Review is one judgement pass: what a pattern cannot decide.
type Review struct {
	ID       string
	Title    string
	Evidence []string
	Ask      string
}

// Reviews is the pack, in the order to run it.
var Reviews = []Review{{
	ID:       "REV-W1",
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
	ID:       "REV-W2",
	Title:    "The concepts the software actually has",
	Evidence: []string{"cat README.md", "cat SPEC.md"},
	Ask: `List the concepts a user has to hold to use this software: the nouns its commands
act on, the relationships between them, and the rules a user cannot infer. Then check each
one against the documents.

Report the concepts the documents name but never explain, the ones they explain twice in
different words, and the ones the software has that they never name at all. Say which
document each belongs in.`,
}, {
	ID:    "REV-W3",
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
}, {
	ID:       "REV-W4",
	Title:    "The claims the code does not support",
	Evidence: []string{"cat MANUAL.md", "cat SPEC.md"},
	Ask: `Read every factual claim the documents make about behaviour: defaults,
precedence, exit codes, what a flag does, what is written where, what is never done.
Confirm each against the source.

Report every claim the code contradicts, and every claim that is true only under a
condition the document does not state. A default that depends on the host is the common
case, and it is usually written as though it were fixed.`,
}, {
	ID:       "REV-W5",
	Title:    "What a run leaves behind",
	Evidence: []string{"cat MANUAL.md"},
	Ask: `For each command that writes anything, list what it creates, where, and what
removes it. Then check the documents say so.

Pay attention to the commands that promise not to write: a dry run, a check, a report, a
verify. Read their implementation and confirm the promise holds all the way down,
including the state the tool keeps for itself. A flag honoured by one layer and ignored by
another is the failure this review exists to find.`,
}, {
	ID:       "REV-W6",
	Title:    "The route nobody takes twice",
	Evidence: []string{"cat INSTALL.md"},
	Ask: `Work through every installation route the page offers, in the order it offers
them, and say for each whether it works today. Run what can be run here. For a route that
cannot be run, name what it depends on and check that dependency exists: a releases page
with an archive on it, a package in a registry, an image in a repository.

A route that cannot work today and does not say so is the finding. It is the first thing
a new reader tries.`,
}}

// RenderReviews returns the pack a model reads.
func (l *Linter) RenderReviews() string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Walkthrough review pack for %s\n\n", l.Tool())
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
