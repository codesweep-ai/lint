package lint

import (
	"strings"
	"testing"
)

func TestSeverityNames(t *testing.T) {
	for _, tc := range []struct {
		s    Severity
		want string
	}{{Error, "error"}, {Warn, "warning"}, {Skip, "skip"}} {
		if got := tc.s.String(); got != tc.want {
			t.Errorf("%d prints %q, want %q", tc.s, got, tc.want)
		}
	}
}

func TestCount(t *testing.T) {
	problems := []Problem{
		Errorf("A-1", "one"), Errorf("A-2", "two"),
		Warnf("B-1", "three"), Skipf("C-1", "four"),
	}
	errors, warnings, skips := Count(problems)
	if errors != 2 || warnings != 1 || skips != 1 {
		t.Errorf("got %d/%d/%d, want 2/1/1", errors, warnings, skips)
	}
}

func TestWaiverOnlyTouchesErrors(t *testing.T) {
	// A warning is already advice, so waiving it would say nothing.
	got := Waive([]Problem{Warnf("A-1", "advice")}, map[string]string{"A-1": "because"})
	if got[0].Severity != Warn {
		t.Errorf("a warning became %s", got[0].Severity)
	}
}

func TestFormatCarriesTheAddress(t *testing.T) {
	p := Errorf("A-1", "something is wrong").At("README.md:12")
	if got := p.Format(); !strings.Contains(got, "README.md:12") {
		t.Errorf("the address is missing: %q", got)
	}
}

func TestSortGroupsTheFamilies(t *testing.T) {
	problems := []Problem{Errorf("OSS-301", "c"), Errorf("OSS-101", "a"), Errorf("OSS-201", "b")}
	SortByRule(problems)
	if problems[0].Rule != "OSS-101" || problems[2].Rule != "OSS-301" {
		t.Errorf("the families are not grouped: %v", problems)
	}
}

func TestRepoReadsAndRemembersAMiss(t *testing.T) {
	r := NewRepo(t.TempDir())
	if _, ok := r.Read("nothing.txt"); ok {
		t.Error("a file that is not there was read")
	}
	// Asking twice costs one failed open.
	if _, ok := r.Read("nothing.txt"); ok {
		t.Error("a file that is not there was read on the second ask")
	}
}
