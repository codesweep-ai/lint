// Package surface checks that the documented interface is the real interface.
//
// Every command a document names, every command the tool carries, every flag
// either side claims, every setting the code reads, every sample output and
// the manual the binary prints: each is compared against the tool itself
// rather than against what a document ought to say.
//
// The comparison needs a binary. Where there is none the rules report a skip,
// one for each, so a run with nothing to ask never reads as a run that
// verified everything.
package surface

import (
	"github.com/codesweep-ai/lint/internal/config"
	"github.com/codesweep-ai/lint/internal/docset"
	"github.com/codesweep-ai/lint/internal/lint"
)

// rule is one interface check.
type rule struct {
	id       string
	severity lint.Severity
	title    string
	why      string
	check    func(*Linter) []lint.Problem
}

// Linter checks one repository's documented interface.
type Linter struct {
	cfg config.Surface
	set *docset.Set
}

// New returns an interface linter reading the document set given.
func New(cfg config.Surface, set *docset.Set) *Linter {
	return &Linter{cfg: cfg, set: set}
}

// Set is the repository the rules read.
func (l *Linter) Set() *docset.Set { return l.set }

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
