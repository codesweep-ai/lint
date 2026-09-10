package surface

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/codesweep-ai/lint/internal/config"
	"github.com/codesweep-ai/lint/internal/docset"
	"github.com/codesweep-ai/lint/internal/lint"
	"github.com/codesweep-ai/lint/internal/lint/linttest"
)

// run applies one rule to a scratch repository holding the files given.
func run(t *testing.T, id string, docs config.Docs, files map[string]string) []lint.Problem {
	t.Helper()
	return runIn(t, id, docs, linttest.Repo(t, files))
}

// runIn is run against a repository the caller has already prepared, which is
// what a rule needing an executable in the tree has to do: linttest writes
// every file unreadable as a program.
func runIn(t *testing.T, id string, docs config.Docs, repo *lint.Repo) []lint.Problem {
	t.Helper()
	if docs.Documents == nil {
		docs.Documents = config.Default().Docs.Documents
	}
	l := New(docs.Surface, docset.New(docs, repo))
	for _, r := range rules {
		if r.id == id {
			return l.runOne(r)
		}
	}
	t.Fatalf("no rule %s", id)
	return nil
}

// fakeTool writes a shell script standing in for the built binary: it answers
// --help with a command list, and prints the manual text given.
func fakeTool(t *testing.T, files map[string]string, manual string) *lint.Repo {
	t.Helper()
	files["bin/tool"] = "#!/bin/sh\ncase \"$1\" in\n" +
		"manual) printf '%s' " + shellQuote(manual) + " ;;\n" +
		"*) printf 'Usage: tool\\n\\nAvailable Commands:\\n  manual  print the manual\\n' ;;\n" +
		"esac\n"
	repo := linttest.Repo(t, files)
	if err := os.Chmod(repo.Path("bin/tool"), 0o755); err != nil {
		t.Fatal(err)
	}
	return repo
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func firstError(problems []lint.Problem) *lint.Problem {
	for i, p := range problems {
		if p.Severity == lint.Error {
			return &problems[i]
		}
	}
	return nil
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

// verbTool writes a binary that carries exactly the verbs given: it lists them
// under --help, answers each of them, and exits non-zero on anything else. That
// last part is what SURF-101 reads, so a fake answering every argument would
// make the rule unable to fail.
func verbTool(t *testing.T, files map[string]string, verbs ...string) *lint.Repo {
	t.Helper()
	var listing, arms strings.Builder
	listing.WriteString(`Usage: tool\n\nAvailable Commands:\n`)
	for _, v := range verbs {
		fmt.Fprintf(&listing, `  %s  what %s does\n`, v, v)
		arms.WriteString(v + "|")
	}
	files["bin/tool"] = "#!/bin/sh\ncase \"$1\" in\n" +
		arms.String() + "-h) printf 'Usage: tool %s\\n' \"$1\" ;;\n" +
		"''|--help|help) printf '" + listing.String() + "' ;;\n" +
		"*) echo \"unknown command \\\"$1\\\" for \\\"tool\\\"\" >&2; exit 1 ;;\n" +
		"esac\n"
	repo := linttest.Repo(t, files)
	if err := os.Chmod(repo.Path("bin/tool"), 0o755); err != nil {
		t.Fatal(err)
	}
	return repo
}

// surfaceDocs is the tuning a SURF-101 case needs: one document, and the fake
// binary to resolve what it names against.
func surfaceDocs(doc string) config.Docs {
	return config.Docs{
		Documents: []string{doc},
		Surface:   config.Surface{Tool: "tool", ToolPath: "bin/tool"},
	}
}

// A spec section states the surface as `tool a|b|c`, which is the compact form
// a man page reaches for and the one where drift is least visible.
func TestStaleAlternativeInASynopsisIsReported(t *testing.T) {
	files := map[string]string{
		"SPEC.md": "# Spec\n\n```sh\ntool doctor|pin|manual|version   # health and metadata\n```\n",
	}
	repo := verbTool(t, files, "doctor", "manual", "version")
	got := runIn(t, "SURF-101", surfaceDocs("SPEC.md"), repo)
	if len(got) != 1 || !strings.Contains(got[0].Message, "`tool pin`") {
		t.Fatalf("got %v, want one finding naming `tool pin`", got)
	}
}

// The same shape with nothing stale in it, which is the case the rule has to
// stay quiet on.
func TestCurrentSynopsisIsNotReported(t *testing.T) {
	files := map[string]string{
		"SPEC.md": "# Spec\n\n```sh\ntool doctor|manual|version\n```\n",
	}
	repo := verbTool(t, files, "doctor", "manual", "version")
	if got := runIn(t, "SURF-101", surfaceDocs("SPEC.md"), repo); len(got) != 0 {
		t.Errorf("a synopsis naming only current verbs reported %v", got)
	}
}

// A bar with no spaces around it is also how a pipeline gets written, and the
// second half of one names another program rather than a verb of this tool.
func TestAPipelineIsNotReadAsASynopsis(t *testing.T) {
	files := map[string]string{
		"SPEC.md": "# Spec\n\n```sh\ntool manual|grep doctor\n```\n",
	}
	repo := verbTool(t, files, "doctor", "manual", "version")
	if got := runIn(t, "SURF-101", surfaceDocs("SPEC.md"), repo); len(got) != 0 {
		t.Errorf("a pipeline was read as a list of alternatives: %v", got)
	}
}

// A slot worn down to two alternatives cannot prove itself: one live verb
// beside one stale one is also the shape of a pipeline written tight. The
// surface block it sits in is what settles the reading, and this is the shape
// a synopsis reaches after verbs are deleted from it one at a time.
func TestDecayedSlotInASurfaceBlockIsReported(t *testing.T) {
	files := map[string]string{
		"SPEC.md": "# Spec\n\n```sh\ntool init|validate|plan\ntool doctor|pin\n```\n",
	}
	repo := verbTool(t, files, "init", "validate", "plan", "doctor")
	got := runIn(t, "SURF-101", surfaceDocs("SPEC.md"), repo)
	if len(got) != 1 || !strings.Contains(got[0].Message, "`tool pin`") {
		t.Fatalf("got %v, want one finding naming `tool pin`", got)
	}
}

// The end of that decay: a slot holding nothing the binary still carries. It
// has no live verb to prove itself with, so only the block can report it.
func TestSlotWhereEveryAlternativeIsStaleIsReported(t *testing.T) {
	files := map[string]string{
		"SPEC.md": "# Spec\n\n```sh\ntool init|validate|plan\ntool pin|frob\n```\n",
	}
	repo := verbTool(t, files, "init", "validate", "plan")
	got := runIn(t, "SURF-101", surfaceDocs("SPEC.md"), repo)
	if len(got) != 2 {
		t.Fatalf("got %v, want a finding for each stale alternative", got)
	}
	for _, want := range []string{"`tool frob`", "`tool pin`"} {
		if !strings.Contains(got[0].Message+got[1].Message, want) {
			t.Errorf("got %v, want one naming %s", got, want)
		}
	}
}

// The block rule cannot be allowed to swallow a pipeline that happens to sit
// in a surface block, which is what the shell-filter guard is for. Each of
// these is a program a document pours a command into, and `rg` is here because
// widening the list past the coreutils is what keeps the guard honest.
func TestAPipelineBesideASynopsisIsNotReadAsOne(t *testing.T) {
	for _, sink := range []string{"grep", "rg", "jq", "less", "xclip"} {
		t.Run(sink, func(t *testing.T) {
			files := map[string]string{
				"SPEC.md": "# Spec\n\n```sh\ntool init|validate|plan\ntool manual|" +
					sink + " doctor\n```\n",
			}
			repo := verbTool(t, files, "init", "validate", "plan", "manual", "doctor")
			if got := runIn(t, "SURF-101", surfaceDocs("SPEC.md"), repo); len(got) != 0 {
				t.Errorf("a pipeline into %s was read as alternatives: %v", sink, got)
			}
		})
	}
}

// The limit of the block rule, asserted so that it stays a decision rather
// than becoming an accident. A lone two-way slot has nothing to prove itself
// against, and reading one as alternatives would report the far side of every
// tight pipeline a document quotes.
func TestALoneTwoWaySlotIsNotReadAsASynopsis(t *testing.T) {
	files := map[string]string{
		"README.md": "# R\n\n```sh\ntool manual|pin\n```\n",
	}
	repo := verbTool(t, files, "manual", "doctor")
	if got := runIn(t, "SURF-101", surfaceDocs("README.md"), repo); len(got) != 0 {
		t.Errorf("a lone two-way slot reported %v", got)
	}
}

// surfaceSectionDocs is surfaceDocs with a section nominated as the one that
// states the command surface.
func surfaceSectionDocs(doc, sect string) config.Docs {
	docs := surfaceDocs(doc)
	docs.Surface.SurfaceSection = sect
	return docs
}

// A verb the rest of the set documents thoroughly is still missing from the
// section that claims to state the surface, which is what SURF-102 cannot see.
func TestSurfaceSectionMissingAVerbIsReported(t *testing.T) {
	files := map[string]string{
		"SPEC.md": "# Spec\n\n### 3.1 The surface\n\n```sh\ntool init|validate\n```\n\n" +
			"### 3.2 Elsewhere\n\n```sh\ntool orientation\n```\n",
	}
	repo := verbTool(t, files, "init", "validate", "orientation")
	got := runIn(t, "SURF-105", surfaceSectionDocs("SPEC.md", "SPEC.md#3.1"), repo)
	if len(got) != 1 || !strings.Contains(got[0].Message, "orientation") {
		t.Fatalf("got %v, want one finding naming orientation", got)
	}
}

// The same section with the verb in it, which is the case the rule has to stay
// quiet on.
func TestCompleteSurfaceSectionIsNotReported(t *testing.T) {
	files := map[string]string{
		"SPEC.md": "# Spec\n\n### 3.1 The surface\n\n```sh\ntool init|validate|orientation\n```\n",
	}
	repo := verbTool(t, files, "init", "validate", "orientation")
	if got := runIn(t, "SURF-105", surfaceSectionDocs("SPEC.md", "SPEC.md#3.1"), repo); len(got) != 0 {
		t.Errorf("a section naming every verb reported %v", got)
	}
}

// A project that has not nominated a section has to learn it lost the check.
func TestSurfaceSectionSkipsWhenNotConfigured(t *testing.T) {
	files := map[string]string{"SPEC.md": "# Spec\n"}
	repo := verbTool(t, files, "init")
	got := runIn(t, "SURF-105", surfaceDocs("SPEC.md"), repo)
	if len(got) != 1 || got[0].Severity != lint.Skip {
		t.Errorf("got %v, want a skip", got)
	}
}

func TestUndocumentedEnvIsReported(t *testing.T) {
	files := map[string]string{
		"main.go":   "package main\nfunc f() { _ = os.Getenv(\"CS_T_SECRET\") }\n",
		"MANUAL.md": "# Manual\n\nNothing about it.\n",
	}
	docs := config.Docs{Surface: config.Surface{EnvPrefix: "CS_T_"}}
	got := run(t, "SURF-201", docs, files)
	if firstError(got) == nil {
		t.Error("a variable the code reads and no document names passed")
	}
	files["MANUAL.md"] = "# Manual\n\nSet `CS_T_SECRET` to change it.\n"
	if firstError(run(t, "SURF-201", docs, files)) != nil {
		t.Error("a documented variable was reported")
	}
}

func TestDocumentedEnvNothingReadsIsReported(t *testing.T) {
	files := map[string]string{
		"main.go":   "package main\nfunc f() { _ = os.Getenv(\"CS_T_REAL\") }\n",
		"MANUAL.md": "# Manual\n\nSet `CS_T_REAL` and `CS_T_GONE`.\n",
	}
	docs := config.Docs{Surface: config.Surface{EnvPrefix: "CS_T_"}}
	got := run(t, "SURF-202", docs, files)
	if len(got) != 1 || !strings.Contains(got[0].Message, "CS_T_GONE") {
		t.Errorf("got %v, want one finding naming CS_T_GONE", got)
	}
}

func TestSafeVerbsAreRequiredBeforeASampleRuns(t *testing.T) {
	// An empty list disables the most valuable rule in the set, and that has to
	// read as a skip rather than as a pass.
	files := map[string]string{"MANUAL.md": "# m\n"}
	got := run(t, "SURF-301", config.Docs{Documents: []string{"MANUAL.md"}}, files)
	if len(got) != 1 || got[0].Severity != lint.Skip {
		t.Errorf("got %v, want a skip", got)
	}
}

func TestEmbeddedManualMatchesTheFile(t *testing.T) {
	const manual = "# The tool manual\n\nWhat it does.\n"
	repo := fakeTool(t, map[string]string{"MANUAL.md": manual}, manual)
	docs := config.Docs{
		Documents: []string{"MANUAL.md"},
		Surface:   config.Surface{Tool: "tool", ToolPath: "bin/tool"},
	}
	if got := runIn(t, "SURF-303", docs, repo); len(got) != 0 {
		t.Errorf("a matching manual was reported as drift: %v", got)
	}
}

func TestEmbeddedManualDriftIsReported(t *testing.T) {
	// The binary is the copy a machine with no checkout reads, so a stale one
	// is the reader who cannot see the tree getting the wrong answer.
	repo := fakeTool(t, map[string]string{"MANUAL.md": "# The tool manual\n\nWhat it does now.\n"},
		"# The tool manual\n\nWhat it did before.\n")
	docs := config.Docs{
		Documents: []string{"MANUAL.md"},
		Surface:   config.Surface{Tool: "tool", ToolPath: "bin/tool"},
	}
	got := runIn(t, "SURF-303", docs, repo)
	if len(got) != 1 || got[0].Severity != lint.Error {
		t.Fatalf("got %v, want one error", got)
	}
	if !strings.Contains(got[0].Message, "line 3") {
		t.Errorf("finding does not name the first differing line: %s", got[0].Message)
	}
}

func TestEmbeddedManualSkipsWithoutTheVerb(t *testing.T) {
	// A tool that does not carry `manual` has nothing to compare, and that has
	// to read as a skip rather than as a pass.
	files := map[string]string{"MANUAL.md": "# m\n"}
	got := run(t, "SURF-303", config.Docs{Documents: []string{"MANUAL.md"}}, files)
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
	if len(seen) != 10 {
		t.Errorf("%d rules are registered, want 10", len(seen))
	}
}

func TestInventoryClassifiesEachCommand(t *testing.T) {
	files := map[string]string{"README.md": "```bash\ncs-t version\ncs-t write\ncd /tmp\nfoo bar\n```\n"}
	docs := config.Docs{
		Documents: []string{"README.md"},
		Surface:   config.Surface{Tool: "cs-t", SafeVerbs: []string{"version"}},
	}
	l := New(docs.Surface, docset.New(docs, linttest.Repo(t, files)))
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

// Every rule here needs the binary, and a checkout that has not built one must
// read as a run that verified nothing rather than as a clean one.
func TestAFullRunSkipsWithoutABinary(t *testing.T) {
	files := map[string]string{
		"README.md":       "# thing\n\n```bash\nthing run\n```\n",
		"INSTALL.md":      "# Install\n\nYou need `git`.\n",
		"MANUAL.md":       "# Manual\n\n## Notes for agents\n\nNon-interactive.\n",
		"SPEC.md":         "# Spec\n\n## 1.1 A section\n",
		"CONTRIBUTING.md": "# Contributing\n\nRun `make check`.\n",
		"main.go":         "package main\n",
	}
	docs := config.Docs{
		Documents: config.Default().Docs.Documents,
		Surface:   config.Surface{Tool: "no-such-tool-anywhere"},
	}
	l := New(docs.Surface, docset.New(docs, linttest.Repo(t, files)))
	got := l.Run()
	skips := 0
	for _, p := range got {
		if strings.Contains(p.Message, "the check itself failed") {
			t.Errorf("%s panicked: %s", p.Rule, p.Message)
		}
		if p.Severity == lint.Error {
			t.Errorf("a repository with no binary reported %v", p)
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

func TestReviewPackRenders(t *testing.T) {
	docs := config.Default().Docs
	l := New(docs.Surface, docset.New(docs, linttest.Repo(t, map[string]string{"a.txt": "x\n"})))
	pack := l.RenderReviews()
	for _, want := range []string{"REV-S1", "REV-S3", "Evidence to gather first"} {
		if !strings.Contains(pack, want) {
			t.Errorf("the pack does not carry %q", want)
		}
	}
}
