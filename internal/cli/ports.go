package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"zlan/internal/transport"
)

func newPortsCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "ports",
		Short: "列出本机可用串口设备",
		Long: `列出本机可用串口设备。macOS 上做串口直连时优先选 /dev/cu.* 而非 /dev/tty.*。

示例:
  zlan ports
  zlan ports --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ports, err := transport.Ports()
			if err != nil {
				return err
			}
			if g.jsonOut {
				return writeJSON(cmd.OutOrStdout(), ports)
			}
			if len(ports) == 0 {
				cmd.PrintErrln("未发现串口设备")
				return nil
			}
			for _, p := range ports {
				fmt.Fprintln(cmd.OutOrStdout(), p)
			}
			return nil
		},
	}
}
