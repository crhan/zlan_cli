package cli

import (
	"github.com/spf13/cobra"

	"zlan/internal/transport"
)

func newTuneCmd(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "tune [target] <field=value>...",
		Short: "临时设置串口参数(不保存、不重启,断电恢复)",
		Long: `临时修改串口相关参数,立即生效但不保存、不重启;设备断电后恢复原值。
适合现场调试波特率/校验位等。需持久化请用 set。

示例:
  zlan tune 192.168.1.200 baud=115200 parity=none
  zlan tune --serial /dev/cu.usbserial-1410 data_bits=8`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWrite(cmd, g, args, transport.WriteVolatile, nil)
		},
	}
}
