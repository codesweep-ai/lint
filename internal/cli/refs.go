package cli

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/codesweep-ai/lint/internal/refs"
)

func refsCmd(opt *options) *cobra.Command {
	var review bool
	cmd := &cobra.Command{
		Use:   "refs",
		Short: "Check that every reference in the documents resolves",
		Long: "Check that everything the documents point at is still there.\n\n" +
			"A path a page names, a section a citation points at, an issue an identifier\n" +
			"promises a record for, a document the router is supposed to list: each is\n" +
			"followed to what it claims to reach. Two neighbours belong here for the same\n" +
			"reason. A block the reader is told to copy, and a program the build needs,\n" +
			"are both something the page hands them, and neither can be followed to\n" +
			"anything that exists.\n\n" +
			"Nothing here runs the tool, so a checkout with no binary built gets the\n" +
			"whole set.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, set, err := documents(opt)
			if err != nil {
				return err
			}
			l := refs.New(cfg.Docs.Refs, set)
			out := cmd.OutOrStdout()

			switch {
			case opt.explain:
				explainRules(out, refs.Explain())
				return nil
			case opt.list:
				listRefs(out, l)
				return nil
			case review:
				fmt.Fprint(out, l.RenderReviews())
				return nil
			}
			report(out, "refs", l.Run(), opt.verbose)
			return nil
		},
	}
	cmd.Flags().BoolVar(&review, "review", false, "print the review pack for the rest")
	cmd.Flags().BoolVar(&opt.explain, "explain", false, "every rule, and what it wants")
	cmd.Flags().BoolVar(&opt.list, "list", false, "what it found to check")
	return cmd
}

func listRefs(w io.Writer, l *refs.Linter) {
	set := l.Set()
	fmt.Fprintf(w, "documents: %v\n", set.Docs())
	fmt.Fprintf(w, "markdown:  %d file(s) scanned for paths\n", len(set.Markdown()))
	fmt.Fprintf(w, "blocks:    %d fenced\n", len(set.Blocks()))
	fmt.Fprintf(w, "cited:     %s\n", orNone(set.CitedByDefault()))
	fmt.Fprintf(w, "rules:     %d\n", len(refs.Explain()))
}
