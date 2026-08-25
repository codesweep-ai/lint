package cli

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/codesweep-ai/lint/internal/surface"
)

func surfaceCmd(opt *options) *cobra.Command {
	var run, review bool
	cmd := &cobra.Command{
		Use:   "surface",
		Short: "Check that the documented interface is the real interface",
		Long: "Check the documents against the binary, the code and the build.\n\n" +
			"It walks the tool's own help tree. It reads the source for the settings the\n" +
			"tool takes, and it re-runs the samples whose commands the tuning file\n" +
			"declares safe. Nothing here guesses what a document ought to say.\n\n" +
			"Every check needs the binary the repository builds, so build first. Without\n" +
			"one each rule reports a skip rather than a pass, which is what lets a\n" +
			"project run this without wiring a build dependency.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, set, err := documents(opt)
			if err != nil {
				return err
			}
			l := surface.New(cfg.Docs.Surface, set)
			out := cmd.OutOrStdout()

			switch {
			case opt.explain:
				explainRules(out, surface.Explain())
				return nil
			case opt.list:
				listSurface(out, l)
				return nil
			case run:
				printInventory(out, l)
				return nil
			case review:
				fmt.Fprint(out, l.RenderReviews())
				return nil
			}
			report(out, "surface", l.Run(), opt.verbose)
			return nil
		},
	}
	cmd.Flags().BoolVar(&run, "run", false, "the ordered inventory of documented commands")
	cmd.Flags().BoolVar(&review, "review", false, "print the review pack for the rest")
	cmd.Flags().BoolVar(&opt.explain, "explain", false, "every rule, and what it wants")
	cmd.Flags().BoolVar(&opt.list, "list", false, "what it found to check")
	return cmd
}

func listSurface(w io.Writer, l *surface.Linter) {
	set := l.Set()
	fmt.Fprintf(w, "tool:      %s\n", set.Tool())
	fmt.Fprintf(w, "binary:    %s\n", orNone(set.Binary()))
	fmt.Fprintf(w, "documents: %v\n", set.Docs())
	fmt.Fprintf(w, "blocks:    %d fenced\n", len(set.Blocks()))
	fmt.Fprintf(w, "verbs:     %d carried\n", len(set.Verbs()))
	fmt.Fprintf(w, "prefix:    %s*\n", set.EnvPrefix())
}

// printInventory prints the checklist a reader works down. Every line is a
// claim that they can follow that command, with the document and line it came
// from, so a finding has an address rather than a paraphrase.
func printInventory(w io.Writer, l *surface.Linter) {
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
