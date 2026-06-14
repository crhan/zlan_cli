package cli

import (
	"runtime/debug"
	"time"

	"github.com/spf13/cobra"
)

// globalFlags 是所有子命令共享的全局开关。
type globalFlags struct {
	serial  string
	baud    int
	jsonOut bool
	quiet   bool
	verbose bool
	noColor bool
	yes     bool
	confirm bool
	timeout time.Duration
	retries int
}

// NewRootCmd 构造根命令与全局 flag。
func NewRootCmd() *cobra.Command {
	g := &globalFlags{}
	root := &cobra.Command{
		Use:   "zlan",
		Short: "管理 ZLAN(卓岚)串口服务器 / 联网模块",
		Long: `zlan —— 通过 UDP 管理端口(1092)或串口命令模式管理卓岚联网模块。

示例:
  zlan discover                              广播发现局域网设备
  zlan info 192.168.1.200                    查看设备完整参数
  zlan set 192.168.1.200 dest_port=4196      改配置(会重启设备)
  zlan info --serial /dev/cu.usbserial-1410  通过串口直连查看`,
		Version:       version(),
		SilenceUsage:  true, // 运行时错误不再喷 usage
		SilenceErrors: true, // 错误由 main 统一打印
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error {
			if g.noColor {
				disableColor()
			}
			return nil
		},
	}

	pf := root.PersistentFlags()
	pf.StringVar(&g.serial, "serial", "", "走串口通道,值为串口设备路径(如 /dev/cu.usbserial-1410)")
	pf.IntVar(&g.baud, "baud", 115200, "串口波特率(须匹配设备当前波特率)")
	pf.BoolVar(&g.jsonOut, "json", false, "输出 JSON(机器可读)")
	pf.BoolVarP(&g.quiet, "quiet", "q", false, "抑制非必要输出")
	pf.BoolVarP(&g.verbose, "verbose", "v", false, "详细输出")
	pf.BoolVar(&g.noColor, "no-color", false, "禁用彩色输出")
	pf.BoolVarP(&g.yes, "yes", "y", false, "跳过确认提示")
	pf.BoolVar(&g.confirm, "confirm", false, "确认执行高危批量 / 网络变更")
	pf.DurationVar(&g.timeout, "timeout", 2*time.Second, "发现 / 单次操作超时")
	pf.IntVar(&g.retries, "retries", 1, "单播重试次数")

	root.AddCommand(
		newDiscoverCmd(g),
		newInfoCmd(g),
		newGetCmd(g),
		newSetCmd(g),
		newTuneCmd(g),
		newRebootCmd(g),
		newStatusCmd(g),
		newMonitorCmd(g),
		newPortsCmd(g),
		newApplyCmd(g),
	)
	return root
}

// version 取构建信息里的版本(go install 带 tag);本地构建回退为 dev。
func version() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
		return bi.Main.Version
	}
	return "dev"
}
