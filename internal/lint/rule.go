package lint

import "strconv"

func itoa(n int) string { return strconv.Itoa(n) }

// RuleDoc is what the explain verb prints: a rule's identity, what it wants,
// and why it exists.
//
// A rule a reader meets in a failure and cannot look up is a rule they can only
// silence, so every linter answers with the same shape and the command tree
// renders them all the same way.
type RuleDoc struct {
	ID       string
	Severity string
	Title    string
	Why      string
}

// Guard runs one rule's check and returns what it found.
//
// It does three things every linter needs identically. A panic becomes a
// finding against the rule that raised it, so one broken check cannot hide the
// findings of the other sixty. A finding that named no rule is stamped with the
// rule that produced it. And a finding cannot report louder than the rule it
// came from, so a rule declared a warning never emits an error.
func Guard(id string, severity Severity, check func() []Problem) (found []Problem) {
	defer func() {
		if v := recover(); v != nil {
			found = []Problem{Warnf(id, "the check itself failed: %v", v)}
		}
	}()
	found = check()
	for i := range found {
		if found[i].Rule == "" {
			found[i].Rule = id
		}
		if found[i].Severity == Error && severity == Warn {
			found[i].Severity = Warn
		}
	}
	return found
}
