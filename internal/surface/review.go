package surface

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/codesweep-ai/lint/internal/docset"
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

// Row is one line of the documented-command inventory.
type Row struct {
	Where   string
	Kind    string // safe, tool, host, other
	Command string
	Note    string
}

// Inventory returns every command the documents tell a reader to run, in
// reading order.
//
// A reader works down it, and so does an agent: each line names the document,
// the line, and the command, so a run can be recorded against a stable address
// rather than a paraphrase.
func (l *Linter) Inventory() []Row {
	tool := l.set.Tool()
	safe := map[string]bool{}
	for _, v := range l.cfg.SafeVerbs {
		safe[v] = true
	}
	var rows []Row
	for _, block := range l.set.Blocks() {
		if !docset.IsShell(block.Lang) {
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
				if safe[docset.VerbOf(words[1:])] {
					kind = "safe"
				}
			case shellNoise[head]:
				kind = "host"
			}
			note := ""
			for _, p := range docset.Placeholders {
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

// Reviews is the pack, in the order to run it. Each asks whether the documents
// tell the truth about the software, which is this linter's question and the
// half of it no pattern can settle.
var Reviews = []Review{{
	ID:       "REV-S1",
	Title:    "The claims the code does not support",
	Evidence: []string{"cat MANUAL.md", "cat SPEC.md"},
	Ask: `Read every factual claim the documents make about behaviour: defaults,
precedence, exit codes, what a flag does, what is written where, what is never done.
Confirm each against the source.

Report every claim the code contradicts, and every claim that is true only under a
condition the document does not state. A default that depends on the host is the common
case, and it is usually written as though it were fixed.`,
}, {
	ID:       "REV-S2",
	Title:    "What a run leaves behind",
	Evidence: []string{"cat MANUAL.md"},
	Ask: `For each command that writes anything, list what it creates, where, and what
removes it. Then check the documents say so.

Pay attention to the commands that promise not to write: a dry run, a check, a report, a
verify. Read their implementation and confirm the promise holds all the way down,
including the state the tool keeps for itself. A flag honoured by one layer and ignored by
another is the failure this review exists to find.`,
}, {
	ID:       "REV-S3",
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
	fmt.Fprintf(&b, "# Interface review pack for %s\n\n", l.set.Tool())
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
