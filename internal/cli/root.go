// Package cli assembles the cs-lint command tree.
package cli

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"

	"github.com/codesweep-ai/lint/internal/config"
	"github.com/codesweep-ai/lint/internal/lint"
)

// devVersion marks a binary that carried no release stamp.
const devVersion = "dev"

// Version is stamped at build time by the release build.
var Version = devVersion

// buildVersion reports the release stamp when there is one, and otherwise the
// module version the toolchain recorded. A binary installed straight from the
// module path carries no stamp, so without this it would answer "dev" and
// leave you guessing which revision produced a finding.
func buildVersion() string {
	if Version != devVersion {
		return Version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok || info.Main.Version == "" || info.Main.Version == "(devel)" {
		return Version
	}
	return info.Main.Version
}

// Exit codes. A gate needs to tell "the repository has a problem" from "the
// linter could not run", because the first is a finding and the second is a
// broken build.
const (
	exitOK       = 0
	exitFound    = 1
	exitBadUsage = 2
)

type options struct {
	root    string
	verbose bool
	explain bool
	list    bool
}

// Execute runs the command tree and returns the process exit code.
func Execute() int {
	opt := &options{}
	root := &cobra.Command{
		Use:   "cs-lint",
		Short: "Check that a repository, its documents and its claims hold together",
		Long: "cs-lint carries three linters over one repository.\n\n" +
			"  docs         how the documents are written\n" +
			"  oss          what a published repository owes a reader\n" +
			"  walkthrough  whether the documents still describe the software\n\n" +
			"Every check is mechanical and quotable. What needs judgement is left to\n" +
			"review, because a linter that guesses produces noise, and noise gets ignored.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&opt.root, "root", ".",
		"the repository to check")
	root.PersistentFlags().BoolVar(&opt.verbose, "verbose", false,
		"report what was skipped, and why")

	root.AddCommand(
		docsCmd(opt),
		ossCmd(opt),
		walkthroughCmd(opt),
		manualCmd(),
		versionCmd(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "cs-lint:", err)
		return exitBadUsage
	}
	return code
}

// code carries the verdict out of a subcommand, because cobra's error channel
// is for usage failures and a finding is not one.
var code = exitOK

func versionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(cmd.OutOrStdout(), "cs-lint", buildVersion())
			return nil
		},
	}
}

// load reads the configuration for a run, so every subcommand reports a broken
// configuration file the same way.
func load(opt *options) (*config.Config, *lint.Repo, error) {
	cfg, err := config.Load(opt.root)
	if err != nil {
		return nil, nil, err
	}
	return cfg, lint.NewRepo(opt.root), nil
}

// report prints what a run found and sets the exit code.
//
// Errors fail the run. Warnings print and pass, because they flag a judgement
// call rather than broken data. Skips print only when asked, but they are
// always counted: a run that verified nothing must never read as a run that
// verified everything.
func report(w io.Writer, name string, problems []lint.Problem, verbose bool) {
	lint.SortByRule(problems)
	for _, p := range problems {
		if p.Severity == lint.Skip && !verbose {
			continue
		}
		fmt.Fprintln(w, p.Format())
		if p.Quote != "" {
			fmt.Fprintf(w, "         %s\n", p.Quote)
		}
	}
	errors, warnings, skips := lint.Count(problems)
	fmt.Fprintf(w, "\n%s: %d error(s), %d warning(s), %d skipped\n",
		name, errors, warnings, skips)
	if errors > 0 {
		code = exitFound
	}
}

// explainRules prints every rule a linter carries, with what it wants and why
// it exists. A rule a reader meets in a failure and cannot look up is a rule
// they can only silence.
func explainRules(w io.Writer, rules []lint.RuleDoc) {
	const indent = "          "
	for _, r := range rules {
		fmt.Fprintf(w, "%s  %-8s %s\n", r.ID, r.Severity, r.Title)
		fmt.Fprintf(w, "%s%s\n\n", indent, wrap(r.Why, 74, indent))
	}
}
