package oss

import (
	"strings"

	"github.com/codesweep-ai/lint/internal/lint"
)

var ledgerRules = []rule{{
	id: "OSS-601", severity: lint.Error,
	title: "A tracked ledger validates and its page is current",
	why: "The rendered page is what a human reads. A record changed without a re-render " +
		"publishes a page that disagrees with the records beside it.",
	check: func(l *Linter) []lint.Problem {
		if !l.keepsLedger() {
			return []lint.Problem{lint.Skipf("OSS-601", "this repository keeps no ledger")}
		}
		if !lint.Have("cs-ledger") {
			return []lint.Problem{lint.Skipf("OSS-601", "cs-ledger is not installed")}
		}
		out, ok := l.repo.Run("cs-ledger", "check", "ledger")
		if !ok {
			return []lint.Problem{lint.Errorf("OSS-601", "cs-ledger check failed: %s", lastLine(out))}
		}
		return nil
	},
}, {
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
