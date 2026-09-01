package oss

import (
	"strings"

	"github.com/codesweep-ai/lint/internal/lint"
)

// Whether a ledger's records are valid and its page current is the question the
// ledger's own tool answers, against the schema the records were written to.
// Asking a second tool would put the verdict on whichever copy of that tool the
// caller happens to have installed, which disagrees with the tree the moment one
// is a version behind. So these rules read what is in the repository: that the
// ledger carries its documents, that the repository routes to it, and that CI
// runs the check.
var ledgerRules = []rule{{
	id: "OSS-602", severity: lint.Error,
	title: "The ledger carries its own router and guide",
	why: "An agent that finds records with no doctrine beside them invents a practice, " +
		"and the ledger fills with records nobody can close.",
	check: func(l *Linter) []lint.Problem {
		if !l.keepsLedger() {
			return []lint.Problem{lint.Skipf("OSS-602", "this repository keeps no ledger")}
		}
		var out []lint.Problem
		for _, name := range []string{"AGENTS.md", "GUIDE.md"} {
			if !l.has("ledger/" + name) {
				out = append(out, lint.Errorf("OSS-602", "ledger/%s is missing", name))
			}
		}
		agents, _ := l.read("ledger/AGENTS.md")
		if agents != "" && len(strings.Split(strings.TrimRight(agents, "\n"), "\n")) > 30 {
			out = append(out, lint.Warnf("OSS-602",
				"ledger/AGENTS.md is not the short router the current cs-ledger writes; re-render it"))
		}
		return out
	},
}, {
	id: "OSS-603", severity: lint.Error,
	title: "The repository points at its own ledger",
	why:   "A ledger nobody is routed to is a ledger nobody files into.",
	check: func(l *Linter) []lint.Problem {
		if !l.keepsLedger() {
			return []lint.Problem{lint.Skipf("OSS-603", "this repository keeps no ledger")}
		}
		var out []lint.Problem
		for _, doc := range []string{"AGENTS.md", "CONTRIBUTING.md"} {
			body, _ := l.read(doc)
			if !strings.Contains(strings.ToLower(body), "ledger") {
				out = append(out, lint.Errorf("OSS-603", "%s never mentions the ledger", doc))
			}
		}
		return out
	},
}, {
	id: "OSS-604", severity: lint.Warn,
	title: "CI gates the ledger",
	why: "CONTRIBUTING asks for a render and a check before every commit that touches the " +
		"ledger. Nothing enforces it until CI does.",
	check: func(l *Linter) []lint.Problem {
		if !l.keepsLedger() {
			return []lint.Problem{lint.Skipf("OSS-604", "this repository keeps no ledger")}
		}
		bodies := l.allWorkflows()
		if strings.Contains(bodies, "cs-ledger") || strings.Contains(bodies, "ledger check") {
			return nil
		}
		return []lint.Problem{lint.Warnf("OSS-604", "no CI job runs cs-ledger check")}
	},
}}
