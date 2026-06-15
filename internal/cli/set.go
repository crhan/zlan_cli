package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"zlan/internal/device"
	"zlan/internal/transport"
)

func newSetCmd(g *globalFlags) *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "set [target] <field=value>...",
		Short: "修改设备配置(保存并重启设备)",
		Long: `修改设备配置并使生效。每个参数形如 field=value(字段名见 zlan get --list-fields)。

⚠ 会让设备保存参数并重启(断开 TCP 连接);改 local_ip/net_mask/gateway/dhcp_en/dns_server_ip
可能使设备切换网段,届时需在新网段重新发现。

示例:
  zlan set 192.168.1.200 dest_port=4196 work_mode=tcp-client
  zlan set 192.168.1.200 --profile modbus-tcp-rtu
  zlan set 192.168.1.200 local_ip=192.168.1.50 net_mask=255.255.255.0 -y
  zlan set --serial /dev/cu.usbserial-1410 baud=115200`,
		RunE: func(cmd *cobra.Command, args []string) error {
			preset, err := profileAssignments(profile)
			if err != nil {
				return exitErr(ExitUsage, err)
			}
			return runWrite(cmd, g, args, transport.WritePersist, preset)
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "套用预设配置(modbus-tcp-rtu)")
	return cmd
}

// runWrite 是 set(持久)与 tune(临时)的共享实现。
func runWrite(cmd *cobra.Command, g *globalFlags, args []string, mode transport.WriteMode, preset map[string]string) error {
	host, kvs, err := parseWriteArgs(g, args, len(preset) > 0)
	if err != nil {
		return exitErr(ExitUsage, err)
	}
	kvs = mergeAssignments(preset, kvs)
	if len(kvs) == 0 {
		return exitErr(ExitUsage, fmt.Errorf("至少给一个 field=value 或 --profile"))
	}
	network := anyNetwork(kvs)

	switch {
	case mode == transport.WriteVolatile && network:
		// tune 定位是串口调试参数(波特率/校验/数据位);网络字段会重启保存,应走 set。
		return exitErr(ExitUsage, fmt.Errorf("tune 不支持网络字段(会重启保存);请用 set 修改 IP/掩码/网关/DHCP/DNS"))
	case mode == transport.WritePersist && network && !g.confirm:
		// SPEC §10:改网络字段可能使设备失联,属高危,-y 不够,必须显式 --confirm。
		return exitErr(ExitUsage, fmt.Errorf("改网络参数(IP/掩码/网关/DHCP/DNS)可能使设备失联,属高危操作,请显式加 --confirm"))
	}

	label := targetLabel(g, host)
	var msg string
	if mode == transport.WritePersist {
		msg = fmt.Sprintf("将修改 %s 的 %d 个字段并保存重启设备", label, len(kvs))
		if network {
			msg += yellow("(含网络参数,设备可能切换网段/失联)")
		}
	} else {
		msg = fmt.Sprintf("将临时修改 %s 的 %d 个串口参数(不保存、断电恢复)", label, len(kvs))
	}
	ok, err := confirm(cmd, g, msg)
	if err != nil {
		return err
	}
	if !ok {
		cmd.PrintErrln("已取消")
		return nil
	}

	return withHost(cmd, g, host, func(ep *device.Endpoint) error {
		res, err := device.Set(ep.Conn, ep.Snapshot, kvs, mode)
		if err != nil {
			return err
		}
		return renderSetResult(cmd, g, res)
	})
}

func profileAssignments(name string) (map[string]string, error) {
	switch name {
	case "":
		return nil, nil
	case "modbus-tcp-rtu":
		return map[string]string{
			"work_mode":  "tcp-server",
			"local_port": "502",
			"app_proto":  "modbus",
			"baud":       "9600",
			"parity":     "none",
			"data_bits":  "8",
		}, nil
	default:
		return nil, fmt.Errorf("未知 profile:%s(可选:modbus-tcp-rtu)", name)
	}
}

func mergeAssignments(base, override map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(override))
	for k, v := range base {
		out[k] = v
	}
	for k, v := range override {
		out[k] = v
	}
	return out
}
