package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"zlan/internal/device"
)

func newStatusCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [target]",
		Short: "查看设备 TCP 连接状态(轻量探针)",
		Long: `读取设备连接状态(status@61 bit0):connected 表示 TCP 已建立或处于 UDP 态。
输出极简,适合脚本轮询(watch zlan status ...)。

示例:
  zlan status 192.168.1.200
  zlan status 192.168.1.200 --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return withEndpoint(cmd, g, args, func(ep *device.Endpoint) error {
				connected, _, err := device.Status(ep.Conn)
				if err != nil {
					return err
				}
				if g.jsonOut {
					return writeJSON(cmd.OutOrStdout(), map[string]bool{"connected": connected})
				}
				label := "idle"
				if connected {
					label = "connected"
				}
				fmt.Fprintln(cmd.OutOrStdout(), label)
				return nil
			})
		},
	}
	cmd.Args = targetArgs(g)
	return cmd
}
