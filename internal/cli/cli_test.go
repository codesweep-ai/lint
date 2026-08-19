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
	code = exitOK
	opt := &options{}
	root_ := &cobra.Command{Use: "cs-lint", SilenceUsage: true, SilenceErrors: true}
	root_.PersistentFlags().StringVar(&opt.root, "root", ".", "")
	root_.PersistentFlags().BoolVar(&opt.verbose, "verbose", false, "")
	root_.AddCommand(docsCmd(opt), ossCmd(opt), walkthroughCmd(opt), manualCmd(), versionCmd())
	var out bytes.Buffer
	root_.SetOut(&out)
	root_.SetErr(&out)
	root_.SetArgs(append([]string{"--root", root}, args...))
	if err := root_.Execute(); err != nil {
		t.Fatalf("%v: %v\n%s", args, err, out.String())
	}
	return out.String(), code
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

func TestDocsReportsAndExitsNonZero(t *testing.T) {
	root := scratch(t, map[string]string{"DOC.md": "The the word is written twice.\n"})
	out, exit := runCLI(t, root, "docs")
	if exit != exitFound {
		t.Errorf("a repository with a finding exited %d, want %d", exit, exitFound)
	}
	if !strings.Contains(out, "DOC-109") {
		t.Errorf("the finding is not in the output: %q", out)
	}
}

func TestDocsPassesOnCleanProse(t *testing.T) {
	root := scratch(t, map[string]string{
		"DOC.md": "# Heading\n\nYou run the gate before you push.\n"})
	out, exit := runCLI(t, root, "docs")
	if exit != exitOK {
		t.Errorf("clean prose exited %d: %s", exit, out)
	}
}

func TestDocsListAndStats(t *testing.T) {
	root := scratch(t, map[string]string{"README.md": "# x\n\nYou read it.\n"})
	out, _ := runCLI(t, root, "docs", "--list")
	if !strings.Contains(out, "README.md") {
		t.Errorf("--list printed %q", out)
	}
	out, _ = runCLI(t, root, "docs", "--stats")
	if !strings.Contains(out, "words") {
		t.Errorf("--stats printed %q", out)
	}
}

func TestExplainNamesTheGuidance(t *testing.T) {
	// A writer arguing with a rule deserves to know whether it is this house's
	// preference or an industry convention with a page behind it.
	out, _ := runCLI(t, t.TempDir(), "docs", "--explain")
	for _, want := range []string{"DOC-101", "DOC-111", "Google", "Red Hat",
		"developers.google.com/style", "redhat-documentation.github.io"} {
		if !strings.Contains(out, want) {
			t.Errorf("--explain does not mention %q", want)
		}
	}
}

func TestOSSAndWalkthroughExplain(t *testing.T) {
	out, _ := runCLI(t, t.TempDir(), "oss", "--explain")
	if !strings.Contains(out, "OSS-101") || !strings.Contains(out, "OSS-803") {
		t.Errorf("oss --explain printed %q", out[:min(len(out), 200)])
	}
	out, _ = runCLI(t, t.TempDir(), "walkthrough", "--explain")
	if !strings.Contains(out, "WALK-101") || !strings.Contains(out, "WALK-602") {
		t.Errorf("walkthrough --explain printed %q", out[:min(len(out), 200)])
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
	out, _ = runCLI(t, t.TempDir(), "walkthrough", "--review")
	if !strings.Contains(out, "REV-W1") || !strings.Contains(out, "REV-W6") {
		t.Error("the walkthrough review pack is incomplete")
	}
}

func TestWalkthroughListAndRun(t *testing.T) {
	root := scratch(t, map[string]string{
		"README.md": "```bash\ncs-lint docs\n```\n",
		"MANUAL.md": "# m\n",
	})
	out, _ := runCLI(t, root, "walkthrough", "--list")
	if !strings.Contains(out, "tool:") {
		t.Errorf("--list printed %q", out)
	}
	out, _ = runCLI(t, root, "walkthrough", "--run")
	if !strings.Contains(out, "documented command") {
		t.Errorf("--run printed %q", out)
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

func TestWalkthroughRuns(t *testing.T) {
	root := scratch(t, map[string]string{
		"AGENTS.md": "# routes\n\nREADME.md\n",
		"README.md": "# x\n",
	})
	out, _ := runCLI(t, root, "walkthrough")
	if !strings.Contains(out, "walkthrough:") {
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
