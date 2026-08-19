// Package lint carries what the three linters have in common: a problem, the
// severity it carries, and the rule that reported it.
//
// A problem is data rather than a printed line, so a caller can count them,
// sort them, waive one by its rule, and render them in whichever form the
// command asked for. Nothing here decides what is wrong; the linters do.
package lint

import (
	"fmt"
	"sort"
	"strings"
)

// Severity says what a problem costs. A run fails on the first Error and
// passes with any number of Warnings, because a warning flags a judgement call
// rather than broken data, and a gate that fails on judgement gets switched off.
type Severity int

const (
	// Error fails the run.
	Error Severity = iota
	// Warn prints and passes.
	Warn
	// Skip records a check that could not run. A run that verified nothing
	// must never read as a run that verified everything, so a skip is
	// reported rather than dropped.
	Skip
)

func (s Severity) String() string {
	switch s {
	case Error:
		return "error"
	case Warn:
		return "warning"
	case Skip:
		return "skip"
	}
	return "unknown"
}

// Problem is one finding: which rule found it, how much it costs, what is
// wrong, and where to look.
type Problem struct {
	Rule     string // the rule id, such as "OSS-102"
	Severity Severity
	Message  string // what is wrong, in one line
	Where    string // the file, or the file and line, that carries it
	Quote    string // the text that proves it, where quoting one helps
}

// Errorf builds an Error-severity problem.
func Errorf(rule, format string, args ...any) Problem {
	return Problem{Rule: rule, Severity: Error, Message: fmt.Sprintf(format, args...)}
}

// Warnf builds a Warn-severity problem.
func Warnf(rule, format string, args ...any) Problem {
	return Problem{Rule: rule, Severity: Warn, Message: fmt.Sprintf(format, args...)}
}

// Skipf builds a Skip-severity problem: a check that could not run, and why.
func Skipf(rule, format string, args ...any) Problem {
	return Problem{Rule: rule, Severity: Skip, Message: fmt.Sprintf(format, args...)}
}

// At returns a copy of the problem carrying the location given.
func (p Problem) At(where string) Problem {
	p.Where = where
	return p
}

// Quoting returns a copy of the problem carrying the text that proves it.
func (p Problem) Quoting(quote string) Problem {
	p.Quote = quote
	return p
}

// Waive downgrades to Skip every Error whose rule the map names, recording the
// reason with it. A waiver with no reason is one nobody can review, so the
// reason travels with the finding and is printed.
func Waive(problems []Problem, allow map[string]string) []Problem {
	out := make([]Problem, 0, len(problems))
	for _, p := range problems {
		if reason, ok := allow[p.Rule]; ok && p.Severity == Error {
			p.Severity = Skip
			p.Message = fmt.Sprintf("%s (waived: %s)", p.Message, reason)
		}
		out = append(out, p)
	}
	return out
}

// Count returns how many problems of each severity the slice holds.
func Count(problems []Problem) (errors, warnings, skips int) {
	for _, p := range problems {
		switch p.Severity {
		case Error:
			errors++
		case Warn:
			warnings++
		case Skip:
			skips++
		}
	}
	return errors, warnings, skips
}

// Format renders one problem as the line a reader sees.
func (p Problem) Format() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%-8s %-9s %s", p.Rule, p.Severity, p.Message)
	if p.Where != "" {
		b.WriteString(" [" + p.Where + "]")
	}
	return b.String()
}

// SortByRule orders problems by rule id, keeping the order they were found in
// within one rule. Reporting groups the families together, which is how the
// ids were numbered to be read.
func SortByRule(problems []Problem) {
	sort.SliceStable(problems, func(i, j int) bool {
		return problems[i].Rule < problems[j].Rule
	})
}
