package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/spf13/cobra"

	"zlan/internal/protocol"
	"zlan/internal/transport"
)

func newMonitorCmd(g *globalFlags) *cobra.Command {
	var listen int
	cmd := &cobra.Command{
		Use:   "monitor",
		Short: "被动监听设备周期上报(0x01)",
		Long: `被动监听设备主动上报的参数包。TCP 客户端 / 定时上报模式下,设备会周期性向目的端口
发送 0x01 应答包。按 Ctrl-C 退出。

示例:
  zlan monitor
  zlan monitor --listen 4196
  zlan monitor --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if g.serial != "" {
				return exitErr(ExitUsage, fmt.Errorf("monitor 是网络监听操作,不能与 --serial 同用"))
			}
			if !g.quiet && !g.jsonOut {
				cmd.PrintErrf("监听 UDP %d 上的设备上报,Ctrl-C 退出...\n", listen)
			}
			out := cmd.OutOrStdout()
			err := transport.Monitor(cmd.Context(), listen, func(src *net.UDPAddr, _ protocol.UDPCmd, p protocol.Param) []byte {
				printReport(out, g, src, &p)
				return nil // 本期只展示,不回包改参(外网改参是后续功能)
			})
			if errors.Is(err, context.Canceled) {
				return nil // Ctrl-C 视为正常退出
			}
			return err
		},
	}
	cmd.Flags().IntVar(&listen, "listen", protocol.MgmtPort, "监听的 UDP 端口")
	return cmd
}

// printReport 打印一条上报(JSON 模式输出 JSONL,每行一个对象)。
func printReport(w io.Writer, g *globalFlags, src *net.UDPAddr, p *protocol.Param) {
	if g.jsonOut {
		_ = writeJSON(w, deviceJSON{
			DevID: get(p, "devid"), Name: get(p, "dev_name"), IP: get(p, "local_ip"),
			Mode: get(p, "work_mode"), Baud: get(p, "baud"), Ver: get(p, "ver"),
			Connected: p.Connected(), Addr: src.String(),
		})
		return
	}
	fmt.Fprintf(w, "[%s] from=%s devid=%s ip=%s mode=%s %s\n",
		time.Now().Format("15:04:05"), src, get(p, "devid"), get(p, "local_ip"), get(p, "work_mode"), statusLabel(p))
}
