package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

func newVersionCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "显示版本信息",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if g.jsonOut {
				return writeJSON(cmd.OutOrStdout(), map[string]string{
					"version": version(),
					"commit":  commit,
					"date":    date,
					"go":      runtime.Version(),
				})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "zlan %s (%s)\n", version(), runtime.Version())
			if commit != "" || date != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "commit %s\nbuilt %s\n", emptyDash(commit), emptyDash(date))
			}
			return nil
		},
	}
}

func emptyDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}
