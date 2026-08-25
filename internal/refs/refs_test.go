package refs

import (
	"strings"
	"testing"

	"github.com/codesweep-ai/lint/internal/config"
	"github.com/codesweep-ai/lint/internal/docset"
	"github.com/codesweep-ai/lint/internal/lint"
	"github.com/codesweep-ai/lint/internal/lint/linttest"
)

// run applies one rule to a scratch repository holding the files given.
//
// The document set lives above both linters in the tuning file, so a test says
// which documents it means through config.Docs and which knobs through
// config.Refs.
func run(t *testing.T, id string, docs config.Docs, cfg config.Refs, files map[string]string) []lint.Problem {
	t.Helper()
	if docs.Documents == nil {
		docs.Documents = config.Default().Docs.Documents
	}
	if cfg.AgentSection == "" {
		cfg.AgentSection = config.Default().Docs.Refs.AgentSection
	}
	docs.Refs = cfg
	l := New(cfg, docset.New(docs, linttest.Repo(t, files)))
	for _, r := range rules {
		if r.id == id {
			return l.runOne(r)
		}
	}
	t.Fatalf("no rule %s", id)
	return nil
}

// set names the documents a case reads, for the common case of one list.
func set(names ...string) config.Docs { return config.Docs{Documents: names} }

func TestNamedPathMustExist(t *testing.T) {
	files := map[string]string{
		"README.md":             "See `internal/real/file.go` and `internal/gone/file.go`.\n",
		"internal/real/file.go": "package real\n",
	}
	got := run(t, "REF-101", set("README.md"), config.Refs{}, files)
	if len(got) != 1 || !strings.Contains(got[0].Message, "internal/gone/file.go") {
		t.Errorf("got %v, want one finding naming the path that is not there", got)
	}
}

func TestNamedPathMustExistOutsideTheDocumentSet(t *testing.T) {
	// A nested README makes the same claim the document set does, and a path
	// it names that has moved is wrong in the same way.
	files := map[string]string{
		"README.md":             "# x\n",
		"fixtures/README.md":    "See `internal/gone/file.go`.\n",
		"internal/real/file.go": "package real\n",
	}
	got := run(t, "REF-101", set("README.md"), config.Refs{}, files)
	if len(got) != 1 || !strings.Contains(got[0].Message, "internal/gone/file.go") {
		t.Fatalf("got %v, want one finding from the nested README", got)
	}
	if !strings.Contains(got[0].Where, "fixtures/README.md") {
		t.Errorf("finding is addressed to %q, want the nested README", got[0].Where)
	}
}

func TestMarkdownSkipExcludesATreeThatClaimsNothing(t *testing.T) {
	// Payload shipped somewhere else names paths in the machine it lands on,
	// not in this repository. A declared prefix says so.
	files := map[string]string{
		"README.md":             "# x\n",
		"image/rootfs/GUIDE.md": "See `internal/gone/file.go`.\n",
	}
	cfg := config.Refs{
		MarkdownSkip: map[string]string{"image/": "shipped into the guest, where its paths resolve"},
	}
	if got := run(t, "REF-101", set("README.md"), cfg, files); len(got) != 0 {
		t.Errorf("a declared skip was scanned anyway: %v", got)
	}
}

func TestMarkdownSkipCannotHideTheDocumentSet(t *testing.T) {
	// The document set is the repository's own claim whatever a prefix says,
	// so a skip that covered it would be a rule deleted in private.
	files := map[string]string{
		"docs/README.md":        "See `internal/gone/file.go`.\n",
		"internal/real/file.go": "package real\n",
	}
	cfg := config.Refs{
		MarkdownSkip: map[string]string{"docs/": "a reason that must not reach the document set"},
	}
	if got := run(t, "REF-101", set("docs/README.md"), cfg, files); len(got) != 1 {
		t.Errorf("got %v, want the document set checked regardless of the skip", got)
	}
}

func TestASymbolCitationIsNotAPath(t *testing.T) {
	// package/file.Symbol is a citation of code, not of a path.
	files := map[string]string{
		"README.md":     "See `internal/x.Linter` for it.\n",
		"internal/x.go": "package x\n",
	}
	if got := run(t, "REF-101", set("README.md"), config.Refs{}, files); len(got) != 0 {
		t.Errorf("a symbol citation was read as a path: %v", got)
	}
}

func TestCitationsResolve(t *testing.T) {
	files := map[string]string{
		"README.md": "See SPEC.md §7.2 and SPEC.md §99.4.\n",
		"SPEC.md":   "# Spec\n\n## 7.2 The thing\n",
	}
	got := run(t, "REF-102", set("README.md", "SPEC.md"), config.Refs{}, files)
	if len(got) != 1 || !strings.Contains(got[0].Message, "99.4") {
		t.Errorf("got %v, want one finding naming the section that is not there", got)
	}
}

func TestSourceCitationsResolve(t *testing.T) {
	files := map[string]string{
		"SPEC.md":       "# Spec\n\n## 7. The thing\n",
		"internal/x.go": "package x\n\n// Holds the rule (SPEC.md §7).\n",
		"internal/y.go": "package y\n\n// Holds the rule (SPEC.md §99.4).\n",
	}
	got := run(t, "REF-103", set("SPEC.md"), config.Refs{}, files)
	if len(got) != 1 || !strings.Contains(got[0].Message, "99.4") {
		t.Fatalf("got %v, want one finding naming the section that is not there", got)
	}
	if !strings.Contains(got[0].Where, "internal/y.go") {
		t.Errorf("finding is addressed to %q, want the source file", got[0].Where)
	}
}

func TestABareCitationReadsAgainstTheSpec(t *testing.T) {
	// A comment writing plain §3 means the document that numbers its sections,
	// which is the spec wherever there is one.
	files := map[string]string{
		"SPEC.md":       "# Spec\n\n## 3. The thing\n",
		"INSTALL.md":    "# Install\n\n## 1. Step\n\n## 9. Step\n",
		"internal/x.go": "package x\n\n// See §9 for it.\n",
	}
	got := run(t, "REF-103", set("SPEC.md", "INSTALL.md"), config.Refs{}, files)
	if len(got) != 1 || !strings.Contains(got[0].Message, "SPEC.md has no section 9") {
		t.Errorf("got %v, want the bare citation read against SPEC.md", got)
	}
}

func TestABoldNumberedSubsectionIsASection(t *testing.T) {
	// A spec numbering its rules as bold lead-ins rather than headings is not
	// a spec whose citations are all stale.
	files := map[string]string{
		"SPEC.md":       "# Spec\n\n## 3. Rules\n\n**3.1 Key order.** It holds.\n",
		"internal/x.go": "package x\n\n// Ordered (SPEC.md §3.1).\n",
	}
	if got := run(t, "REF-103", set("SPEC.md"), config.Refs{}, files); len(got) != 0 {
		t.Errorf("a bold-numbered subsection was read as missing: %v", got)
	}
}

func TestCitationSkipExcludesADeclaredTree(t *testing.T) {
	files := map[string]string{
		"SPEC.md":       "# Spec\n\n## 7. The thing\n",
		"internal/x.go": "package x\n\n// Quotes SPEC.md §99.4 to explain one.\n",
	}
	cfg := config.Refs{
		CitationSkip: map[string]string{"internal/": "the rules quote the citations they match"},
	}
	if got := run(t, "REF-103", set("SPEC.md"), cfg, files); len(got) != 0 {
		t.Errorf("a declared skip was scanned anyway: %v", got)
	}
}

func TestABareCitationIsLeftAloneWhenAmbiguous(t *testing.T) {
	// Two numbered documents and no spec: a rule that guesses which was meant
	// reports a finding nobody can act on.
	files := map[string]string{
		"INSTALL.md":    "# Install\n\n## 1. Step\n",
		"MANUAL.md":     "# Manual\n\n## 1. Verb\n",
		"internal/x.go": "package x\n\n// See §99 for it.\n",
	}
	got := run(t, "REF-103", set("INSTALL.md", "MANUAL.md"), config.Refs{}, files)
	if len(got) != 0 {
		t.Errorf("an ambiguous bare citation was resolved anyway: %v", got)
	}
}

func TestAPlaceholderPathIsReported(t *testing.T) {
	// A block introduced as runnable, opening on a repository the reader was
	// never given, fails on its first line.
	files := map[string]string{"README.md": "```bash\ncd ~/my-service\ntool run\n```\n"}
	got := run(t, "REF-201", set("README.md"), config.Refs{}, files)
	if len(got) != 1 || !strings.Contains(got[0].Message, "~/my-service") {
		t.Fatalf("got %v, want one finding naming the placeholder", got)
	}
	cfg := config.Refs{PlaceholderOK: []string{"my-service"}}
	if got := run(t, "REF-201", set("README.md"), cfg, files); len(got) != 0 {
		t.Errorf("a declared placeholder was reported anyway: %v", got)
	}
}

func TestPrerequisitesAreNamed(t *testing.T) {
	files := map[string]string{
		"Makefile":   "build:\n\t@command -v goreleaser >/dev/null\n\t@command -v cosign >/dev/null\n",
		"INSTALL.md": "# Install\n\nYou need `goreleaser`.\n",
	}
	got := run(t, "REF-202", set("INSTALL.md"), config.Refs{}, files)
	if len(got) != 1 || !strings.Contains(got[0].Message, "cosign") {
		t.Errorf("got %v, want one finding naming cosign", got)
	}
}

func TestManualAnswersTheAutomatedCaller(t *testing.T) {
	files := map[string]string{"MANUAL.md": "# Manual\n\nNothing for an agent.\n"}
	if len(run(t, "REF-301", set("MANUAL.md"), config.Refs{}, files)) == 0 {
		t.Error("a manual with no agent section passed")
	}
	files["MANUAL.md"] = "# Manual\n\n## Notes for agents\n\nEverything is non-interactive.\n"
	got := run(t, "REF-301", set("MANUAL.md"), config.Refs{}, files)
	for _, p := range got {
		if p.Severity != lint.Skip {
			t.Errorf("a manual with an agent section reported %v", p)
		}
	}
}

func TestRouterNamesEveryDocument(t *testing.T) {
	files := map[string]string{
		"AGENTS.md": "This file routes to [README.md](README.md).\n",
		"README.md": "# x\n",
		"MANUAL.md": "# m\n",
	}
	got := run(t, "REF-302", set("README.md", "MANUAL.md"), config.Refs{}, files)
	if len(got) != 1 || !strings.Contains(got[0].Message, "MANUAL.md") {
		t.Errorf("got %v, want one finding naming MANUAL.md", got)
	}
}

// A citation is a promise that somewhere there is an account of why the code is
// shaped that way. Nothing else catches a broken one: a ledger reports on
// itself, and the citations live outside it. Measured on a repository whose
// ledger was reset for publication, where fourteen identifiers in help strings
// and requirements went on naming records that had been removed.
func TestCitedIssuesHaveRecords(t *testing.T) {
	files := map[string]string{
		"ledger/ledger.json":             `{"project":"x","idPrefix":"SAC"}`,
		"ledger/issues/SAC-001.json":     `{"id":"SAC-001"}`,
		"MANUAL.md":                      "# Manual\n\nAudit the evidence (SAC-009).\n",
		"internal/x.go":                  "package x\n\n// The pin is recorded deliberately (SAC-006).\nvar X = 1\n",
		"ledger/issues/SAC-002.json.bak": `{"id":"SAC-002"}`,
	}
	got := run(t, "REF-303", set("MANUAL.md"), config.Refs{}, files)
	if len(got) != 2 {
		t.Fatalf("got %v, want SAC-006 and SAC-009 both reported", got)
	}
	joined := got[0].Message + " " + got[1].Message
	for _, want := range []string{"SAC-006", "SAC-009"} {
		if !strings.Contains(joined, want) {
			t.Errorf("%s was not reported: %v", want, got)
		}
	}

	// The record that does exist is not reported, and neither is the ledger's
	// own mention of an identifier it holds no record for.
	files["MANUAL.md"] = "# Manual\n\nThe first record is SAC-001.\n"
	files["internal/x.go"] = "package x\n\n// See SAC-001.\nvar X = 1\n"
	files["ledger/queue.json"] = `{"items":[{"id":"SAC-404"}]}`
	if got := run(t, "REF-303", set("MANUAL.md"), config.Refs{}, files); len(got) != 0 {
		t.Errorf("a citation with a record, or one inside the ledger, was reported: %v", got)
	}
}

// A repository with no ledger is not a repository with broken citations.
func TestCitedIssuesSkipsWithoutALedger(t *testing.T) {
	files := map[string]string{"MANUAL.md": "# Manual\n\nSomething about SAC-009.\n"}
	got := run(t, "REF-303", set("MANUAL.md"), config.Refs{}, files)
	if len(got) != 1 || got[0].Severity != lint.Skip {
		t.Errorf("got %v, want a skip", got)
	}
}

func TestEveryRuleIsExplained(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range Explain() {
		if seen[r.ID] {
			t.Errorf("%s is listed twice", r.ID)
		}
		seen[r.ID] = true
		if r.Title == "" || r.Why == "" {
			t.Errorf("%s has no title or no reason", r.ID)
		}
	}
	if len(seen) != 8 {
		t.Errorf("%d rules are registered, want 8", len(seen))
	}
}

// Nothing here runs the tool, so a checkout with no binary built gets the whole
// set. That is what lets a gate run the reference rules before the build.
func TestAFullRunNeedsNoBinary(t *testing.T) {
	files := map[string]string{
		"README.md":       "# thing\n\n```bash\nthing run\n```\n",
		"INSTALL.md":      "# Install\n\nYou need `git`.\n",
		"MANUAL.md":       "# Manual\n\n## Notes for agents\n\nNon-interactive.\n",
		"SPEC.md":         "# Spec\n\n## 1.1 A section\n",
		"CONTRIBUTING.md": "# Contributing\n\nRun `make check`.\n",
		"AGENTS.md": "# Working here\n\nThis routes to README.md, INSTALL.md, MANUAL.md, " +
			"SPEC.md and CONTRIBUTING.md.\n",
		"Makefile": "build:\n\t@command -v git >/dev/null\n",
		"main.go":  "package main\n",
	}
	cfg := config.Default().Docs
	l := New(cfg.Refs, docset.New(cfg, linttest.Repo(t, files)))
	for _, p := range l.Run() {
		if strings.Contains(p.Message, "the check itself failed") {
			t.Errorf("%s panicked: %s", p.Rule, p.Message)
		}
		if p.Severity == lint.Error {
			t.Errorf("a clean tree reported %v", p)
		}
	}
}

func TestReviewPackRenders(t *testing.T) {
	cfg := config.Default().Docs
	l := New(cfg.Refs, docset.New(cfg, linttest.Repo(t, map[string]string{"a.txt": "x\n"})))
	pack := l.RenderReviews()
	for _, want := range []string{"REV-R1", "REV-R3", "Evidence to gather first"} {
		if !strings.Contains(pack, want) {
			t.Errorf("the pack does not carry %q", want)
		}
	}
}
