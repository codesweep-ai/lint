package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/codesweep-ai/lint/internal/lint"
	"github.com/codesweep-ai/lint/internal/prose"
)

func proseCmd(opt *options) *cobra.Command {
	var stats bool
	cmd := &cobra.Command{
		Use:   "prose",
		Short: "Check how the documents are written",
		Long: "Check the prose in this repository's Markdown against the writing rules.\n\n" +
			"The rules exist because docs drift into a style that reads as terse and\n" +
			"knowing rather than clear: verbless epigrams, sentences carrying two or\n" +
			"three em-dashes, and terms used pages before anything defines them.\n\n" +
			"Code fences, tables and link definitions are excluded throughout: they are\n" +
			"not prose, and none of the rules are about them.\n\n" +
			"It asks for no binary and no build, so it is the first gate a repository\n" +
			"can run and the cheapest one to keep green.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, repo, err := load(opt)
			if err != nil {
				return err
			}
			l, err := prose.New(cfg.Docs.Prose)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()

			if opt.explain {
				explainProse(out)
				return nil
			}

			files, err := l.Files(repo.Root)
			if err != nil {
				return err
			}
			if opt.list {
				for _, f := range files {
					fmt.Fprintln(out, f)
				}
				return nil
			}
			if len(files) == 0 {
				fmt.Fprintln(out, "prose: no Markdown found")
				return nil
			}

			var problems []lint.Problem
			for _, f := range files {
				found, err := l.Check(repo.Root, f)
				if err != nil {
					return err
				}
				problems = append(problems, found...)
			}
			report(out, "prose", problems, opt.verbose)

			if stats {
				printStats(out, l, repo.Root, files)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&stats, "stats", false, "print the per-file measurements")
	cmd.Flags().BoolVar(&opt.explain, "explain", false, "every rule, and what it wants")
	cmd.Flags().BoolVar(&opt.list, "list", false, "show which files are checked")
	return cmd
}

// explainProse prints every rule, what it wants, and the guidance behind it.
//
// A writer arguing with a rule deserves to know whether it is this house's
// preference or an industry convention with a page behind it, so the rules
// that follow published guidance name it.
func explainProse(w io.Writer) {
	for _, r := range prose.Rules {
		source := "this house, from review of these documents"
		switch r.Source {
		case prose.Both:
			source = "the Google and Red Hat documentation style guides"
		case prose.Google, prose.RedHat:
			source = "the " + string(r.Source) + " documentation style guide"
		}
		fmt.Fprintf(w, "%s  %s\n", r.ID, r.Title)
		fmt.Fprintf(w, "          %s\n", wrap(r.Why, 74, "          "))
		fmt.Fprintf(w, "          Follows: %s\n\n", source)
	}
	fmt.Fprintln(w, "  Google:  https://developers.google.com/style")
	fmt.Fprintln(w, "  Red Hat: https://redhat-documentation.github.io/supplementary-style-guide/")
}

// wrap breaks a paragraph to a width, indenting every line after the first.
func wrap(s string, width int, indent string) string {
	var out, line []string
	n := 0
	for word := range strings.FieldsSeq(s) {
		if n > 0 && n+1+len(word) > width {
			out = append(out, strings.Join(line, " "))
			line, n = nil, 0
		}
		line = append(line, word)
		n += len(word) + 1
	}
	if len(line) > 0 {
		out = append(out, strings.Join(line, " "))
	}
	return strings.Join(out, "\n"+indent)
}

func printStats(w io.Writer, l *prose.Linter, root string, files []string) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, "\nfile\twords\tavg sentence\tem-dash/100w\tyou\t")
	for _, f := range files {
		s, err := l.Stats(root, f)
		if err != nil {
			continue
		}
		fmt.Fprintf(tw, "%s\t%d\t%.1f\t%.2f\t%d\t\n", s.File, s.Words, s.AvgLength, s.EmDashes, s.You)
	}
	tw.Flush()
}
