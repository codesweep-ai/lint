package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// runCLI drives one subcommand against a scratch repository and returns what it
// printed and the exit code it set.
func runCLI(t *testing.T, root string, args ...string) (string, int) {
	t.Helper()
	out, exit, err := tryCLI(t, root, args...)
	if err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out)
	}
	return out, exit
}

// tryCLI is runCLI for the cases where failing is the point: a command that
// was removed, or a tuning file the tool refuses to run against.
func tryCLI(t *testing.T, root string, args ...string) (string, int, error) {
	t.Helper()
	code = exitOK
	opt := &options{}
	tree := &cobra.Command{Use: "cs-lint", SilenceUsage: true, SilenceErrors: true}
	tree.PersistentFlags().StringVar(&opt.root, "root", ".", "")
	tree.PersistentFlags().BoolVar(&opt.verbose, "verbose", false, "")
	tree.AddCommand(proseCmd(opt), refsCmd(opt), surfaceCmd(opt), ossCmd(opt),
		walkthroughCmd(), manualCmd(), versionCmd())
	var out bytes.Buffer
	tree.SetOut(&out)
	tree.SetErr(&out)
	tree.SetArgs(append([]string{"--root", root}, args...))
	err := tree.Execute()
	if err != nil {
		return out.String(), exitBadUsage, err
	}
	return out.String(), code, nil
}

func scratch(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for name, body := range files {
		full := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestVersionPrints(t *testing.T) {
	out, _ := runCLI(t, t.TempDir(), "version")
	if !strings.Contains(out, "cs-lint") {
		t.Errorf("version printed %q", out)
	}
}

func TestManualPrints(t *testing.T) {
	out, _ := runCLI(t, t.TempDir(), "manual")
	if out == "" {
		t.Error("the manual verb printed nothing")
	}
}

func TestProseReportsAndExitsNonZero(t *testing.T) {
	root := scratch(t, map[string]string{"DOC.md": "The the word is written twice.\n"})
	out, exit := runCLI(t, root, "prose")
	if exit != exitFound {
		t.Errorf("a repository with a finding exited %d, want %d", exit, exitFound)
	}
	if !strings.Contains(out, "PROSE-109") {
		t.Errorf("the finding is not in the output: %q", out)
	}
}

func TestProsePassesOnCleanProse(t *testing.T) {
	root := scratch(t, map[string]string{
		"DOC.md": "# Heading\n\nYou run the gate before you push.\n"})
	out, exit := runCLI(t, root, "prose")
	if exit != exitOK {
		t.Errorf("clean prose exited %d: %s", exit, out)
	}
}

func TestProseListAndStats(t *testing.T) {
	root := scratch(t, map[string]string{"README.md": "# x\n\nYou read it.\n"})
	out, _ := runCLI(t, root, "prose", "--list")
	if !strings.Contains(out, "README.md") {
		t.Errorf("--list printed %q", out)
	}
	out, _ = runCLI(t, root, "prose", "--stats")
	if !strings.Contains(out, "words") {
		t.Errorf("--stats printed %q", out)
	}
}

func TestExplainNamesTheGuidance(t *testing.T) {
	// A writer arguing with a rule deserves to know whether it is this house's
	// preference or an industry convention with a page behind it.
	out, _ := runCLI(t, t.TempDir(), "prose", "--explain")
	for _, want := range []string{"PROSE-101", "PROSE-111", "Google", "Red Hat",
		"developers.google.com/style", "redhat-documentation.github.io"} {
		if !strings.Contains(out, want) {
			t.Errorf("--explain does not mention %q", want)
		}
	}
}

func TestEveryLinterExplainsItsRules(t *testing.T) {
	for _, tc := range []struct{ verb, first, last string }{
		{"oss", "OSS-101", "OSS-803"},
		{"refs", "REF-101", "REF-303"},
		{"surface", "SURF-101", "SURF-303"},
	} {
		out, _ := runCLI(t, t.TempDir(), tc.verb, "--explain")
		if !strings.Contains(out, tc.first) || !strings.Contains(out, tc.last) {
			t.Errorf("%s --explain printed %q", tc.verb, out[:min(len(out), 200)])
		}
	}
}

func TestReviewPacksRender(t *testing.T) {
	out, _ := runCLI(t, t.TempDir(), "oss", "--review")
	if !strings.Contains(out, "REV-01") || !strings.Contains(out, "REV-08") {
		t.Error("the readiness review pack is incomplete")
	}
	if !strings.Contains(out, "Report only") {
		t.Error("the pack does not say it changes nothing")
	}
	// The pack is a document on standard output and nothing more, so it names
	// no tool that would act on it.
	if strings.Contains(out, "--agent") || strings.Contains(out, "--fix") ||
		strings.Contains(out, "claude") {
		t.Error("the pack still refers to a tool cs-lint does not drive")
	}
	out, _ = runCLI(t, t.TempDir(), "refs", "--review")
	if !strings.Contains(out, "REV-R1") || !strings.Contains(out, "REV-R3") {
		t.Error("the reference review pack is incomplete")
	}
	out, _ = runCLI(t, t.TempDir(), "surface", "--review")
	if !strings.Contains(out, "REV-S1") || !strings.Contains(out, "REV-S3") {
		t.Error("the interface review pack is incomplete")
	}
}

func TestSurfaceListAndRun(t *testing.T) {
	root := scratch(t, map[string]string{
		"README.md": "```bash\ncs-lint prose\n```\n",
		"MANUAL.md": "# m\n",
	})
	out, _ := runCLI(t, root, "surface", "--list")
	if !strings.Contains(out, "tool:") {
		t.Errorf("--list printed %q", out)
	}
	out, _ = runCLI(t, root, "surface", "--run")
	if !strings.Contains(out, "documented command") {
		t.Errorf("--run printed %q", out)
	}
}

func TestRefsListPrintsWhatItFound(t *testing.T) {
	root := scratch(t, map[string]string{"README.md": "# x\n"})
	out, _ := runCLI(t, root, "refs", "--list")
	for _, want := range []string{"documents:", "blocks:", "rules:"} {
		if !strings.Contains(out, want) {
			t.Errorf("--list does not print %q: %s", want, out)
		}
	}
}

func TestABrokenConfigIsReported(t *testing.T) {
	root := scratch(t, map[string]string{".cs-lint.yaml": "oss:\n  proejct: x\n"})
	code = exitOK
	opt := &options{root: root}
	if _, _, err := load(opt); err == nil {
		t.Error("a misspelled knob was accepted")
	}
}

// A waiver whose identifier matches nothing is a rule switched off in private,
// which is what requiring a reason exists to prevent. It stops the run.
func TestAnUnknownWaiverStopsTheRun(t *testing.T) {
	for _, tc := range []struct{ body, names string }{
		{"oss:\n  allow:\n    OSS-999: \"no such rule\"\n", "not a rule cs-lint carries"},
		{"docs:\n  refs:\n    allow:\n      WALK-302: \"renumbered\"\n", "REF-101"},
		{"docs:\n  surface:\n    allow:\n      DOC-104: \"renamed\"\n", "PROSE-104"},
		{"docs:\n  refs:\n    allow:\n      SURF-101: \"wrong section\"\n", "docs.surface.allow"},
	} {
		root := scratch(t, map[string]string{".cs-lint.yaml": tc.body})
		out, exit, err := tryCLI(t, root, "oss")
		if err == nil {
			t.Errorf("%s was accepted: %s", tc.body, out)
			continue
		}
		if exit != exitBadUsage {
			t.Errorf("%s exited %d, want %d", tc.body, exit, exitBadUsage)
		}
		if !strings.Contains(err.Error(), tc.names) {
			t.Errorf("%q does not name %q", err, tc.names)
		}
	}
}

// A waiver naming a rule the linter carries is applied rather than reported.
func TestAKnownWaiverIsAccepted(t *testing.T) {
	root := scratch(t, map[string]string{
		".cs-lint.yaml": "docs:\n  refs:\n    allow:\n      REF-302: \"the router is generated\"\n",
		"README.md":     "# x\n",
	})
	if _, _, err := tryCLI(t, root, "refs"); err != nil {
		t.Errorf("a waiver naming a real rule was rejected: %v", err)
	}
}

// The command that was split answers by name, and exits 2 rather than 1: a
// gate still calling it is broken rather than failing.
func TestTheSplitCommandNamesItsSuccessors(t *testing.T) {
	out, exit, err := tryCLI(t, t.TempDir(), "walkthrough")
	if err == nil {
		t.Fatalf("the removed command ran: %s", out)
	}
	if exit != exitBadUsage {
		t.Errorf("it exited %d, want %d", exit, exitBadUsage)
	}
	for _, want := range []string{"cs-lint surface", "cs-lint refs"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q does not name %s", err, want)
		}
	}
}

func TestVerboseShowsTheSkips(t *testing.T) {
	// A run that verified nothing must never read as a run that verified
	// everything, so a skip is reported when asked for.
	root := scratch(t, map[string]string{"README.md": "# x\n"})
	quiet, _ := runCLI(t, root, "oss")
	loud, _ := runCLI(t, root, "oss", "--verbose")
	if strings.Count(loud, "skip") <= strings.Count(quiet, "skip") {
		t.Error("--verbose did not report what was skipped")
	}
}

func TestWrapKeepsTheIndent(t *testing.T) {
	got := wrap("one two three four five", 9, "  ")
	if !strings.Contains(got, "\n  ") {
		t.Errorf("wrap did not indent the continuation: %q", got)
	}
}

func TestOSSListPrintsWhatItFound(t *testing.T) {
	root := scratch(t, map[string]string{"README.md": "# x\n"})
	out, _ := runCLI(t, root, "oss", "--list")
	for _, want := range []string{"project:", "tracked:", "rules:"} {
		if !strings.Contains(out, want) {
			t.Errorf("--list does not print %q: %s", want, out)
		}
	}
}

func TestOSSRunsAndReports(t *testing.T) {
	// A repository with no licence and no documents fails, which is what makes
	// the summary line worth printing.
	root := scratch(t, map[string]string{"a.txt": "x\n"})
	out, exit := runCLI(t, root, "oss")
	if exit != exitFound {
		t.Errorf("a bare repository exited %d, want %d", exit, exitFound)
	}
	if !strings.Contains(out, "OSS-101") {
		t.Errorf("the missing licence is not reported: %s", out)
	}
	if !strings.Contains(out, "error(s)") {
		t.Errorf("no summary line: %s", out)
	}
}

func TestRefsRuns(t *testing.T) {
	root := scratch(t, map[string]string{
		"AGENTS.md": "# routes\n\nREADME.md\n",
		"README.md": "# x\n",
	})
	out, _ := runCLI(t, root, "refs")
	if !strings.Contains(out, "refs:") {
		t.Errorf("no summary line: %s", out)
	}
}

func TestSurfaceRuns(t *testing.T) {
	root := scratch(t, map[string]string{"README.md": "# x\n", "MANUAL.md": "# m\n"})
	out, _ := runCLI(t, root, "surface")
	if !strings.Contains(out, "surface:") {
		t.Errorf("no summary line: %s", out)
	}
}

func TestTheReviewPackDrivesNothing(t *testing.T) {
	// What a reader does with a finding is theirs to decide, so no flag offers
	// to act on one.
	cmd := ossCmd(&options{})
	for _, gone := range []string{"agent", "fix"} {
		if cmd.Flags().Lookup(gone) != nil {
			t.Errorf("the oss command still carries a --%s flag", gone)
		}
	}
}
