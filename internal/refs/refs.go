// Package refs checks that every reference in the documents, and every
// reference to them, resolves.
//
// A path a document names, a section a citation points at, an issue an
// identifier promises a record for, a document the router is supposed to list:
// each is followed to what it claims to reach. Two neighbouring checks belong
// here for the same reason: a block the reader is told to copy, and a program
// the build needs, are both something the page hands them and neither can be
// followed to anything that exists.
//
// Nothing here runs the tool. Every check is a static read of the tree, so a
// repository with no binary built gets the whole set.
package refs

import (
	"strings"

	"github.com/codesweep-ai/lint/internal/config"
	"github.com/codesweep-ai/lint/internal/docset"
	"github.com/codesweep-ai/lint/internal/lint"
)

// rule is one reference check.
type rule struct {
	id       string
	severity lint.Severity
	title    string
	why      string
	check    func(*Linter) []lint.Problem
}

// Linter checks one repository's references.
type Linter struct {
	cfg config.Refs
	set *docset.Set
}

// New returns a reference linter reading the document set given.
func New(cfg config.Refs, set *docset.Set) *Linter {
	return &Linter{cfg: cfg, set: set}
}

// Set is the repository the rules read.
func (l *Linter) Set() *docset.Set { return l.set }

// skipCitations reports whether a declared prefix covers this file.
func (l *Linter) skipCitations(name string) bool {
	for prefix := range l.cfg.CitationSkip {
		if strings.HasPrefix(name, prefix) {
			return true
		}
	}
	return false
}

// Run applies every rule and returns what they found, with any waiver applied.
func (l *Linter) Run() []lint.Problem {
	var out []lint.Problem
	for _, r := range rules {
		out = append(out, l.runOne(r)...)
	}
	return lint.Waive(out, l.cfg.Allow)
}

func (l *Linter) runOne(r rule) []lint.Problem {
	return lint.Guard(r.id, r.severity, func() []lint.Problem { return r.check(l) })
}

// Explain returns every rule, what it wants, and why it exists.
func Explain() []lint.RuleDoc {
	out := make([]lint.RuleDoc, 0, len(rules))
	for _, r := range rules {
		out = append(out, lint.RuleDoc{
			ID: r.id, Severity: r.severity.String(), Title: r.title, Why: r.why})
	}
	return out
}
