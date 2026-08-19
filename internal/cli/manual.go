package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	lintdoc "github.com/codesweep-ai/lint"
)

func manualCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "manual",
		Short: "Print the manual",
		Long: "Print MANUAL.md, which is compiled into the binary. A machine with the tool\n" +
			"has the reference, with no checkout and no network.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprint(cmd.OutOrStdout(), lintdoc.ManualMD)
			return nil
		},
	}
}
