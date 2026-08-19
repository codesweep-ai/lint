package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/codesweep-ai/lint/internal/oss"
)

func ossCmd(opt *options) *cobra.Command {
	var online, review bool
	cmd := &cobra.Command{
		Use:   "oss",
		Short: "Check that this repository can be published",
		Long: "Check that this repository is in a shape it can be published in.\n\n" +
			"The rules are what a published project owes a reader: a licence, a document\n" +
			"set, a build a stranger's clone can run, a release they can verify, and\n" +
			"nothing in the tree or in any past commit that was never meant to leave the\n" +
			"machine it was written on.\n\n" +
			"Every pattern matches a class rather than a name, so nothing private is\n" +
			"written down: a username is the segment after /home/ that is not a\n" +
			"placeholder the project ships, and the name of whoever runs the check comes\n" +
			"from the environment. A term no pattern can infer goes in .leakterms at the\n" +
			"root, which is gitignored.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, repo, err := load(opt)
			if err != nil {
				return err
			}
			l := oss.New(cfg.OSS, repo)
			l.Online = online
			out := cmd.OutOrStdout()

			switch {
			case opt.explain:
				explainRules(out, oss.Explain())
				return nil
			case opt.list:
				listOSS(out, l, repo.Tracked())
				return nil
			case review:
				fmt.Fprint(out, l.RenderReviews())
				return nil
			}
			report(out, "oss", l.Run(), opt.verbose)
			return nil
		},
	}
	cmd.Flags().BoolVar(&online, "online", false, "ask the forge about the repository itself")
	cmd.Flags().BoolVar(&review, "review", false, "print the review pack for what a pattern cannot decide")
	cmd.Flags().BoolVar(&opt.explain, "explain", false, "every rule, and what it wants")
	cmd.Flags().BoolVar(&opt.list, "list", false, "what it found to check")
	return cmd
}

func listOSS(w io.Writer, l *oss.Linter, tracked []string) {
	fmt.Fprintf(w, "project:  %s\n", l.Project())
	fmt.Fprintf(w, "remote:   %s\n", orNone(l.Slug()))
	fmt.Fprintf(w, "tracked:  %d file(s)\n", len(tracked))
	fmt.Fprintf(w, "rules:    %d\n", len(oss.Explain()))
}

func orNone(s string) string {
	if s == "" {
		return "(none)"
	}
	return s
}
