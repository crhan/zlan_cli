package cli

import (
	"github.com/spf13/cobra"

	"zlan/internal/device"
)

func newInfoCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "info [target]",
		Short: "读取并展示设备完整参数",
		Long: `读取设备完整参数并分组展示。target 为 IP 或 DevID(MAC);用 --serial 时省略 target。

示例:
  zlan info 192.168.1.200
  zlan info 5a:4c:6f:73:cc:d6
  zlan info --serial /dev/cu.usbserial-1410`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return withEndpoint(cmd, g, args, func(ep *device.Endpoint) error {
				p, err := readParam(ep)
				if err != nil {
					return err
				}
				return renderParam(cmd.OutOrStdout(), &p, g.jsonOut)
			})
		},
	}
	cmd.Args = targetArgs(g)
	return cmd
}
