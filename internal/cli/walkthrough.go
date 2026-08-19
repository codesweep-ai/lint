package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/codesweep-ai/lint/internal/walk"
)

func walkthroughCmd(opt *options) *cobra.Command {
	var run, review bool
	cmd := &cobra.Command{
		Use:     "walkthrough",
		Aliases: []string{"walk"},
		Short:   "Check the docs against the binary, the code and the build",
		Long: "Check that the documentation still describes the software it ships with.\n\n" +
			"Every check compares a document against something that cannot lie: the tool's\n" +
			"own help tree, the source that reads an environment variable, the build file\n" +
			"that shells out to a binary, or the command re-run right now. Nothing here\n" +
			"guesses what a document ought to say.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, repo, err := load(opt)
			if err != nil {
				return err
			}
			l := walk.New(cfg.Walkthrough, repo)
			out := cmd.OutOrStdout()

			switch {
			case opt.explain:
				explainRules(out, walk.Explain())
				return nil
			case opt.list:
				listWalk(out, l)
				return nil
			case run:
				printInventory(out, l)
				return nil
			case review:
				fmt.Fprint(out, l.RenderReviews())
				return nil
			}
			report(out, "walkthrough", l.Run(), opt.verbose)
			return nil
		},
	}
	cmd.Flags().BoolVar(&run, "run", false, "the ordered inventory of documented commands")
	cmd.Flags().BoolVar(&review, "review", false, "print the review pack for the rest")
	cmd.Flags().BoolVar(&opt.explain, "explain", false, "every rule, and what it wants")
	cmd.Flags().BoolVar(&opt.list, "list", false, "what it found to check")
	return cmd
}

func listWalk(w io.Writer, l *walk.Linter) {
	fmt.Fprintf(w, "tool:    %s\n", l.Tool())
	fmt.Fprintf(w, "binary:  %s\n", orNone(l.Binary()))
	fmt.Fprintf(w, "docs:    %v\n", l.Docs())
	fmt.Fprintf(w, "blocks:  %d fenced\n", len(l.Blocks()))
	fmt.Fprintf(w, "verbs:   %d carried\n", len(l.Verbs()))
	fmt.Fprintf(w, "prefix:  %s*\n", l.EnvPrefix())
}

// printInventory prints the checklist a walkthrough works down. Every line is
// a claim that a reader can follow that command, with the document and line it
// came from, so a finding has an address rather than a paraphrase.
func printInventory(w io.Writer, l *walk.Linter) {
	rows := l.Inventory()
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for _, r := range rows {
		// The note column is written only when there is one, so a run with no
		// placeholders does not pad every line to the width of the widest.
		if r.Note == "" {
			fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Where, r.Kind, r.Command)
			continue
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.Where, r.Kind, r.Command, r.Note)
	}
	tw.Flush()
	fmt.Fprintf(w, "\n%d documented command(s)\n", len(rows))
}
