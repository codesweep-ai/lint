package walk

import (
	"strings"
	"testing"

	"github.com/codesweep-ai/lint/internal/config"
	"github.com/codesweep-ai/lint/internal/lint"
	"github.com/codesweep-ai/lint/internal/lint/linttest"
)

func run(t *testing.T, id string, cfg config.Walkthrough, files map[string]string) []lint.Problem {
	t.Helper()
	base := config.Default().Walkthrough
	if cfg.Docs == nil {
		cfg.Docs = base.Docs
	}
	if cfg.AgentSection == "" {
		cfg.AgentSection = base.AgentSection
	}
	l := New(cfg, linttest.Repo(t, files))
	for _, r := range rules {
		if r.id == id {
			return l.runOne(r)
		}
	}
	t.Fatalf("no rule %s", id)
	return nil
}

func firstError(problems []lint.Problem) *lint.Problem {
	for i, p := range problems {
		if p.Severity == lint.Error {
			return &problems[i]
		}
	}
	return nil
}

func TestBlockSplitsAConsoleSample(t *testing.T) {
	// In a console block the lines after a prompt are the command, and
	// everything else is the output the document claims it prints.
	b := newBlock("MANUAL.md", 1, "console", "$ tool version\ntool 1.2.3\n$ tool ls\na\nb\n")
	if len(b.Commands) != 2 {
		t.Fatalf("got %d commands, want 2: %v", len(b.Commands), b.Commands)
	}
	if b.Commands[0] != "tool version" || len(b.Output[0]) != 1 {
		t.Errorf("first sample is %q -> %v", b.Commands[0], b.Output[0])
	}
	if len(b.Output[1]) != 2 {
		t.Errorf("second sample's output is %v, want two lines", b.Output[1])
	}
}

func TestBlockJoinsAContinuedLine(t *testing.T) {
	b := newBlock("README.md", 1, "bash", "tool run \\\n  --flag value\n")
	if len(b.Commands) != 1 {
		t.Fatalf("got %d commands, want 1: %v", len(b.Commands), b.Commands)
	}
	if !strings.Contains(b.Commands[0], "--flag value") {
		t.Errorf("the continuation was lost: %q", b.Commands[0])
	}
}

func TestVerbOfDropsFlagsAndTheirValues(t *testing.T) {
	// A flag's value is a word like any other, so `--cassette build` would read
	// as a verb called build.
	for _, tc := range []struct{ in, want string }{
		{"cassette ls", "cassette ls"},
		{"--cassette build verify", "verify"},
		{"--json=true report", "report"},
		// A short flag carries no "=", so the word after it is its value.
		{"-v status", ""},
	} {
		if got := verbOf(strings.Fields(tc.in)); got != tc.want {
			t.Errorf("verbOf(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestMatchesTreatsAnElisionAsAWildcard(t *testing.T) {
	// An elision buys drift in one place rather than turning the whole line off.
	if !matches("wrote page.html (… bytes, 10 issues)", "wrote page.html (4096 bytes, 10 issues)") {
		t.Error("an elision did not match what moved")
	}
	if matches("wrote page.html (… bytes, 10 issues)", "wrote page.html (4096 bytes, 11 issues)") {
		t.Error("an elision matched a part that means something")
	}
	if !matches("exact line", "exact line") {
		t.Error("an exact line did not match itself")
	}
	if matches("exact line", "different line") {
		t.Error("an exact line matched something else")
	}
}

func TestVerbsInReadsACommandsSection(t *testing.T) {
	// The last row is the widest name on the page, so its description is one
	// space away. Requiring two silently drops the longest verb of every help
	// page, and with it every flag and subcommand underneath.
	help := "Usage:\n  tool [command]\n\nAvailable Commands:\n" +
		"  record        record a session\n" +
		"  normalize <path>    write the JSON tree\n" +
		"  help          Help about any command\n" +
		"  walkthrough check the docs against the binary\n\nFlags:\n  -h, --help\n"
	got := verbsIn(help)
	want := map[string]bool{"record": true, "normalize": true, "help": true,
		"walkthrough": true}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for _, v := range got {
		if !want[v] {
			t.Errorf("found %q, which is not a verb here", v)
		}
	}
	// help and completion sort last, so an ordinary verb is discovered first.
	if got[len(got)-1] != "help" {
		t.Errorf("help is not last: %v", got)
	}
}

func TestEnvReadsIgnoresAWriteSite(t *testing.T) {
	// A tool that hands a child process -e NAME=value is not reading NAME.
	body := "cmd := exec.Command(\"x\", \"-e\", \"CS_T_CHILD=1\")\n" +
		"v := os.Getenv(\"CS_T_REAL\")\n"
	got := envReads("CS_T_", body)
	if !got["CS_T_REAL"] {
		t.Error("a getenv-shaped read was missed")
	}
	if got["CS_T_CHILD"] {
		t.Error("a write site was read as a setting")
	}
}

func TestEnvReadsFindsAShellVariable(t *testing.T) {
	if got := envReads("CS_T_", "echo $CS_T_ROOT\n"); !got["CS_T_ROOT"] {
		t.Error("a $NAME read was missed")
	}
}

func TestIsTestRecognisesTheConventions(t *testing.T) {
	for _, p := range []string{"internal/x_test.go", "test/helper.go", "tests/a.py",
		"spec/b.rb", "src/test_thing.py", "web/a.spec.ts"} {
		if !isTest(p) {
			t.Errorf("%s is test code and was not recognised", p)
		}
	}
	for _, p := range []string{"internal/x.go", "cmd/tool/main.go", "latest/thing.go"} {
		if isTest(p) {
			t.Errorf("%s is not test code and was recognised as it", p)
		}
	}
}

func TestUndocumentedEnvIsReported(t *testing.T) {
	files := map[string]string{
		"main.go":   "package main\nfunc f() { _ = os.Getenv(\"CS_T_SECRET\") }\n",
		"MANUAL.md": "# Manual\n\nNothing about it.\n",
	}
	got := run(t, "WALK-201", config.Walkthrough{EnvPrefix: "CS_T_"}, files)
	if firstError(got) == nil {
		t.Error("a variable the code reads and no document names passed")
	}
	files["MANUAL.md"] = "# Manual\n\nSet `CS_T_SECRET` to change it.\n"
	if firstError(run(t, "WALK-201", config.Walkthrough{EnvPrefix: "CS_T_"}, files)) != nil {
		t.Error("a documented variable was reported")
	}
}

func TestDocumentedEnvNothingReadsIsReported(t *testing.T) {
	files := map[string]string{
		"main.go":   "package main\nfunc f() { _ = os.Getenv(\"CS_T_REAL\") }\n",
		"MANUAL.md": "# Manual\n\nSet `CS_T_REAL` and `CS_T_GONE`.\n",
	}
	got := run(t, "WALK-202", config.Walkthrough{EnvPrefix: "CS_T_"}, files)
	if len(got) != 1 || !strings.Contains(got[0].Message, "CS_T_GONE") {
		t.Errorf("got %v, want one finding naming CS_T_GONE", got)
	}
}

func TestNamedPathMustExist(t *testing.T) {
	files := map[string]string{
		"README.md":             "See `internal/real/file.go` and `internal/gone/file.go`.\n",
		"internal/real/file.go": "package real\n",
	}
	got := run(t, "WALK-302", config.Walkthrough{Docs: []string{"README.md"}}, files)
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
	got := run(t, "WALK-302", config.Walkthrough{Docs: []string{"README.md"}}, files)
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
	cfg := config.Walkthrough{
		Docs:         []string{"README.md"},
		MarkdownSkip: map[string]string{"image/": "shipped into the guest, where its paths resolve"},
	}
	if got := run(t, "WALK-302", cfg, files); len(got) != 0 {
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
	cfg := config.Walkthrough{
		Docs:         []string{"docs/README.md"},
		MarkdownSkip: map[string]string{"docs/": "a reason that must not reach the document set"},
	}
	if got := run(t, "WALK-302", cfg, files); len(got) != 1 {
		t.Errorf("got %v, want the document set checked regardless of the skip", got)
	}
}

func TestASymbolCitationIsNotAPath(t *testing.T) {
	// package/file.Symbol is a citation of code, not of a path.
	files := map[string]string{
		"README.md":     "See `internal/x.Linter` for it.\n",
		"internal/x.go": "package x\n",
	}
	if got := run(t, "WALK-302", config.Walkthrough{Docs: []string{"README.md"}}, files); len(got) != 0 {
		t.Errorf("a symbol citation was read as a path: %v", got)
	}
}

func TestCitationsResolve(t *testing.T) {
	files := map[string]string{
		"README.md": "See SPEC.md §7.2 and SPEC.md §99.4.\n",
		"SPEC.md":   "# Spec\n\n## 7.2 The thing\n",
	}
	got := run(t, "WALK-303", config.Walkthrough{Docs: []string{"README.md", "SPEC.md"}}, files)
	if len(got) != 1 || !strings.Contains(got[0].Message, "99.4") {
		t.Errorf("got %v, want one finding naming the section that is not there", got)
	}
}

func TestRouterNamesEveryDocument(t *testing.T) {
	files := map[string]string{
		"AGENTS.md": "This file routes to [README.md](README.md).\n",
		"README.md": "# x\n",
		"MANUAL.md": "# m\n",
	}
	got := run(t, "WALK-602", config.Walkthrough{Docs: []string{"README.md", "MANUAL.md"}}, files)
	if len(got) != 1 || !strings.Contains(got[0].Message, "MANUAL.md") {
		t.Errorf("got %v, want one finding naming MANUAL.md", got)
	}
}

func TestManualAnswersTheAutomatedCaller(t *testing.T) {
	files := map[string]string{"MANUAL.md": "# Manual\n\nNothing for an agent.\n"}
	if len(run(t, "WALK-601", config.Walkthrough{Docs: []string{"MANUAL.md"}}, files)) == 0 {
		t.Error("a manual with no agent section passed")
	}
	files["MANUAL.md"] = "# Manual\n\n## Notes for agents\n\nEverything is non-interactive.\n"
	got := run(t, "WALK-601", config.Walkthrough{Docs: []string{"MANUAL.md"}}, files)
	for _, p := range got {
		if p.Severity != lint.Skip {
			t.Errorf("a manual with an agent section reported %v", p)
		}
	}
}

func TestPrerequisitesAreNamed(t *testing.T) {
	files := map[string]string{
		"Makefile":   "build:\n\t@command -v goreleaser >/dev/null\n\t@command -v cosign >/dev/null\n",
		"INSTALL.md": "# Install\n\nYou need `goreleaser`.\n",
	}
	got := run(t, "WALK-501", config.Walkthrough{Docs: []string{"INSTALL.md"}}, files)
	if len(got) != 1 || !strings.Contains(got[0].Message, "cosign") {
		t.Errorf("got %v, want one finding naming cosign", got)
	}
}

func TestSafeVerbsAreRequiredBeforeASampleRuns(t *testing.T) {
	// An empty list disables the most valuable rule in the set, and that has to
	// read as a skip rather than as a pass.
	files := map[string]string{"MANUAL.md": "# m\n"}
	got := run(t, "WALK-401", config.Walkthrough{Docs: []string{"MANUAL.md"}}, files)
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
	if len(seen) != 14 {
		t.Errorf("%d rules are registered, want 14", len(seen))
	}
}

func TestInventoryClassifiesEachCommand(t *testing.T) {
	files := map[string]string{"README.md": "```bash\ncs-t version\ncs-t write\ncd /tmp\nfoo bar\n```\n"}
	l := New(config.Walkthrough{Docs: []string{"README.md"}, Tool: "cs-t",
		SafeVerbs: []string{"version"}}, linttest.Repo(t, files))
	rows := l.Inventory()
	want := []string{"safe", "tool", "host", "other"}
	if len(rows) != len(want) {
		t.Fatalf("got %d rows, want %d: %v", len(rows), len(want), rows)
	}
	for i, w := range want {
		if rows[i].Kind != w {
			t.Errorf("row %d is %q, want %q (%s)", i, rows[i].Kind, w, rows[i].Command)
		}
	}
}

func TestAFullRunSurvivesEveryRepository(t *testing.T) {
	// Every rule runs against a repository with documents and no binary, which
	// is the state half these checks have to report a skip for.
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
	l := New(config.Default().Walkthrough, linttest.Repo(t, files))
	got := l.Run()
	skips := 0
	for _, p := range got {
		if strings.Contains(p.Message, "the check itself failed") {
			t.Errorf("%s panicked: %s", p.Rule, p.Message)
		}
		if p.Severity == lint.Skip {
			skips++
		}
	}
	if skips == 0 {
		t.Error("no rule reported a skip with no binary present, so a run that " +
			"asked the binary nothing read as complete")
	}
}

func TestHelpTreeIsEmptyWithoutABinary(t *testing.T) {
	l := New(config.Walkthrough{Tool: "no-such-tool-anywhere"}, linttest.Repo(t,
		map[string]string{"a.txt": "x\n"}))
	if l.Binary() != "" {
		t.Errorf("found a binary that is not there: %q", l.Binary())
	}
	if len(l.HelpTree()) != 0 {
		t.Errorf("walked a help tree with no binary: %v", l.HelpTree())
	}
	if len(l.Verbs()) != 0 || len(l.Flags()) != 0 {
		t.Error("verbs or flags came from nowhere")
	}
}

func TestEnvPrefixIsDerivedFromTheTool(t *testing.T) {
	l := New(config.Walkthrough{Tool: "cs-thing"}, linttest.Repo(t, map[string]string{"a.txt": "x\n"}))
	if got := l.EnvPrefix(); got != "CS_THING_" {
		t.Errorf("prefix derived as %q, want CS_THING_", got)
	}
	l = New(config.Walkthrough{Tool: "cs-thing", EnvPrefix: "OTHER_"},
		linttest.Repo(t, map[string]string{"a.txt": "x\n"}))
	if got := l.EnvPrefix(); got != "OTHER_" {
		t.Errorf("the configured prefix was ignored: %q", got)
	}
}

func TestReviewPackRenders(t *testing.T) {
	l := New(config.Default().Walkthrough, linttest.Repo(t, map[string]string{"a.txt": "x\n"}))
	pack := l.RenderReviews()
	for _, want := range []string{"REV-W1", "REV-W6", "Evidence to gather first"} {
		if !strings.Contains(pack, want) {
			t.Errorf("the pack does not carry %q", want)
		}
	}
}

func TestBlocksAreFoundInEveryDocument(t *testing.T) {
	files := map[string]string{
		"README.md": "```bash\none\n```\n\ntext\n\n```console\n$ two\nout\n```\n",
		"MANUAL.md": "```\nthree\n```\n",
	}
	l := New(config.Walkthrough{Docs: []string{"README.md", "MANUAL.md"}}, linttest.Repo(t, files))
	if got := len(l.Blocks()); got != 3 {
		t.Errorf("found %d blocks, want 3", got)
	}
}
