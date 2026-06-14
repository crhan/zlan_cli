package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"zlan/internal/device"
)

func newRebootCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "reboot [target]",
		Short: "重启设备",
		Long: `重启设备(会断开其当前 TCP 连接)。target 为 IP 或 DevID(MAC);用 --serial 时省略。

示例:
  zlan reboot 192.168.1.200
  zlan reboot 192.168.1.200 -y`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ok, err := confirm(cmd, g, fmt.Sprintf("将重启设备 %s(断开其 TCP 连接)", targetLabel(g, hostArg(g, args))))
			if err != nil {
				return err
			}
			if !ok {
				cmd.PrintErrln("已取消")
				return nil
			}
			return withEndpoint(cmd, g, args, func(ep *device.Endpoint) error {
				if err := ep.Conn.Reboot(); err != nil {
					return err
				}
				cmd.PrintErrln(green("重启命令已发送。"))
				return nil
			})
		},
	}
	cmd.Args = targetArgs(g)
	return cmd
}
