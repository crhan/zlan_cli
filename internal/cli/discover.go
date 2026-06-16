package cli

import (
	"fmt"
	"net"
	"time"

	"github.com/spf13/cobra"

	"zlan/internal/device"
	"zlan/internal/protocol"
	"zlan/internal/transport"
)

func newDiscoverCmd(g *globalFlags) *cobra.Command {
	var bind string
	var bindPort int
	var targets []string
	cmd := &cobra.Command{
		Use:     "discover",
		Aliases: []string{"scan", "ls"},
		Short:   "发现局域网内所有 ZLAN 设备",
		Long: `发现局域网内所有 ZLAN 设备。

默认按每个本机 IPv4 网段发送 UDP 广播;对较小的本地网段还会补充单播探测,
用于发现不响应广播但响应单播查询的设备。

示例:
  zlan discover
  zlan discover --timeout 5s
  zlan discover --target 192.168.1.255 --bind 192.168.1.10
  zlan discover --json | jq '.[].ip'`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if g.serial != "" {
				return exitErr(ExitUsage, fmt.Errorf("discover 是网络发现操作,不能与 --serial 同用"))
			}
			opt, err := parseDiscoverOptions(g.timeout, bind, bindPort, targets)
			if err != nil {
				return exitErr(ExitUsage, err)
			}
			if !g.quiet && !g.jsonOut {
				cmd.PrintErrf("正在发现设备(UDP %d,等待 %s)...\n", protocol.MgmtPort, g.timeout)
			}
			devs, err := device.DiscoverWithOptions(cmd.Context(), opt)
			if err != nil {
				return err
			}
			if len(devs) == 0 {
				if g.jsonOut {
					_ = renderDevices(cmd.OutOrStdout(), devs, true) // 仍输出合法空数组 []
				}
				return exitErr(ExitNotFound, fmt.Errorf(
					"未发现任何设备(确认设备上电、与本机同一局域网、防火墙放行 UDP %d)", protocol.MgmtPort))
			}
			if !g.quiet && !g.jsonOut {
				cmd.PrintErrf("发现 %d 台设备\n", len(devs))
			}
			return renderDevices(cmd.OutOrStdout(), devs, g.jsonOut)
		},
	}
	cmd.Flags().StringSliceVar(&targets, "target", nil, "广播目标 IP(可重复或逗号分隔;默认自动按网段定向广播)")
	cmd.Flags().StringVar(&bind, "bind", "", "本地绑定 IPv4 地址(默认 0.0.0.0)")
	cmd.Flags().IntVar(&bindPort, "bind-port", 0, "本地 UDP 源端口(默认随机端口)")
	return cmd
}

func parseDiscoverOptions(wait time.Duration, bind string, bindPort int, targets []string) (transport.DiscoverOptions, error) {
	if bindPort < 0 || bindPort > 65535 {
		return transport.DiscoverOptions{}, fmt.Errorf("--bind-port 需在 0..65535")
	}
	opt := transport.DiscoverOptions{Wait: wait, BindPort: bindPort}
	if bind != "" {
		ip := net.ParseIP(bind).To4()
		if ip == nil {
			return transport.DiscoverOptions{}, fmt.Errorf("--bind 需为 IPv4 地址:%s", bind)
		}
		opt.BindIP = ip
	}
	for _, raw := range targets {
		ip := net.ParseIP(raw).To4()
		if ip == nil {
			return transport.DiscoverOptions{}, fmt.Errorf("--target 需为 IPv4 地址:%s", raw)
		}
		opt.Targets = append(opt.Targets, ip)
	}
	return opt, nil
}
