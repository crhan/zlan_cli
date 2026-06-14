package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"zlan/internal/device"
	"zlan/internal/protocol"
)

// target 把全局 flag + 目标主机组装成 device.Target。
func (g *globalFlags) target(host string) device.Target {
	return device.Target{
		Serial:  g.serial,
		Baud:    g.baud,
		Host:    host,
		Timeout: g.timeout,
		Retries: g.retries,
	}
}

// hostArg 取位置参数作为目标(IP/DevID);串口模式无目标。
func hostArg(g *globalFlags, args []string) string {
	if g.serial != "" || len(args) == 0 {
		return ""
	}
	return args[0]
}

// targetArgs 校验目标参数:串口模式不接受目标,否则恰好一个(SPEC §5)。
func targetArgs(g *globalFlags) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if g.serial != "" {
			if len(args) > 0 {
				return fmt.Errorf("已用 --serial 走串口,不应再给设备目标 %q", args[0])
			}
			return nil
		}
		if len(args) != 1 {
			return fmt.Errorf("需要一个设备目标(IP 或 DevID/MAC),或用 --serial 走串口")
		}
		return nil
	}
}

// targetLabel 是目标的人类可读标签(用于确认提示)。
func targetLabel(g *globalFlags, host string) string {
	if g.serial != "" {
		return "串口 " + g.serial
	}
	if host == "" {
		return "设备"
	}
	return host
}

// withHost 解析目标、建立连接、执行 fn,并负责关闭连接。
func withHost(cmd *cobra.Command, g *globalFlags, host string, fn func(*device.Endpoint) error) error {
	ep, err := g.target(host).Open(cmd.Context())
	if err != nil {
		return err
	}
	defer ep.Conn.Close()
	return fn(ep)
}

// withEndpoint 是 withHost 的位置参数适配版。
func withEndpoint(cmd *cobra.Command, g *globalFlags, args []string, fn func(*device.Endpoint) error) error {
	return withHost(cmd, g, hostArg(g, args), fn)
}

// readParam 优先用解析阶段带回的快照(按 DevID 发现时),否则现读。
func readParam(ep *device.Endpoint) (protocol.Param, error) {
	if ep.Snapshot != nil {
		return *ep.Snapshot, nil
	}
	return ep.Conn.ReadParam()
}

// parseWriteArgs 解析 set/tune 的参数:[target] field=value...,并校验字段名存在。
func parseWriteArgs(g *globalFlags, args []string) (host string, kvs map[string]string, err error) {
	kvArgs := args
	if g.serial == "" {
		if len(args) < 1 {
			return "", nil, fmt.Errorf("需要设备目标(IP 或 DevID/MAC)")
		}
		host = args[0]
		kvArgs = args[1:]
	}
	if len(kvArgs) == 0 {
		return "", nil, fmt.Errorf("至少给一个 field=value")
	}
	kvs = make(map[string]string, len(kvArgs))
	for _, a := range kvArgs {
		k, v, ok := strings.Cut(a, "=")
		if !ok || k == "" {
			return "", nil, fmt.Errorf("参数格式应为 field=value:%q", a)
		}
		if _, ok := protocol.FieldByName(k); !ok {
			if _, ok := protocol.BitFieldByName(k); !ok {
				return "", nil, fmt.Errorf("未知字段:%s(用 zlan get --list-fields 查看)", k)
			}
		}
		kvs[k] = v
	}
	return host, kvs, nil
}

// anyNetwork 报告 kvs 是否含会触发重启/换网段的网络字段。
func anyNetwork(kvs map[string]string) bool {
	for k := range kvs {
		if device.IsNetworkField(k) {
			return true
		}
	}
	return false
}
