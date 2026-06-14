package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"zlan/internal/device"
	"zlan/internal/protocol"
)

func newDiscoverCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:     "discover",
		Aliases: []string{"scan", "ls"},
		Short:   "广播发现局域网内所有 ZLAN 设备",
		Long: `广播发现局域网内所有 ZLAN 设备。

示例:
  zlan discover
  zlan discover --timeout 5s
  zlan discover --json | jq '.[].ip'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if g.serial != "" {
				return exitErr(ExitUsage, fmt.Errorf("discover 是广播操作,不能与 --serial 同用"))
			}
			if !g.quiet && !g.jsonOut {
				cmd.PrintErrf("正在广播发现(UDP %d,等待 %s)...\n", protocol.MgmtPort, g.timeout)
			}
			devs, err := device.Discover(cmd.Context(), g.timeout)
			if err != nil {
				return err
			}
			if len(devs) == 0 && !g.jsonOut {
				return exitErr(ExitNotFound, fmt.Errorf(
					"未发现任何设备(确认设备上电、与本机同一局域网、防火墙放行 UDP %d)", protocol.MgmtPort))
			}
			if !g.quiet && !g.jsonOut {
				cmd.PrintErrf("发现 %d 台设备\n", len(devs))
			}
			return renderDevices(cmd.OutOrStdout(), devs, g.jsonOut)
		},
	}
}
