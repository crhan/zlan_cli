package cli

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"zlan/internal/device"
	"zlan/internal/modbus"
	"zlan/internal/protocol"
)

type regOptions struct {
	unit     uint
	mode     string
	dataHost string
	dataPort int
	kind     string
}

type regDataPath struct {
	Address  string
	Mode     modbus.Mode
	WorkMode string
	AppProto string
	Warnings []string
}

func newRegCmd(g *globalFlags) *cobra.Command {
	opt := &regOptions{unit: 1, mode: "auto", kind: "holding"}
	cmd := &cobra.Command{
		Use:     "reg",
		Aliases: []string{"regs", "register", "modbus", "mb"},
		Short:   "通过 ZLAN 数据通道读写 Modbus 数据点",
		Long: `通过指定 ZLAN 设备的当前参数自动选择寄存器访问方式:
- app_proto=modbus:使用 Modbus TCP
- app_proto=transparent:使用 Modbus RTU 帧透传到 TCP 数据通道

当前只主动连接 tcp-server 模式设备。tcp-client/udp 模式没有本机可主动打开的
数据 TCP 监听,需要先调整设备配置或在对应服务器侧操作。`,
	}
	cmd.PersistentFlags().UintVar(&opt.unit, "unit", 1, "Modbus 从站地址(1..247)")
	cmd.PersistentFlags().StringVar(&opt.mode, "mode", "auto", "数据通道协议:auto|modbus-tcp|rtu-over-tcp")
	cmd.PersistentFlags().StringVar(&opt.dataHost, "data-host", "", "覆盖数据通道主机/IP(默认取设备 local_ip)")
	cmd.PersistentFlags().IntVar(&opt.dataPort, "data-port", 0, "覆盖数据通道端口(默认取设备 local_port)")

	cmd.AddCommand(newRegReadCmd(g, opt), newRegWriteCmd(g, opt), newRegSessionCmd(g, opt), newRegPollCmd(g, opt))
	return cmd
}

func newRegReadCmd(g *globalFlags, opt *regOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "read <target> <addr> [count]",
		Short: "读取 Modbus 寄存器、线圈或离散输入",
		Example: `  zlan reg read 192.168.15.42 0x0001 2 --unit 11
  zlan reg read 192.168.15.42 0x0001 --kind input --unit 11 --json
  zlan reg read 192.168.15.47 0x0001 --kind coil --unit 133`,
		Args: func(_ *cobra.Command, args []string) error {
			if g.serial != "" {
				return fmt.Errorf("reg 通过 ZLAN 网络数据通道读写 Modbus 数据点,不能与 --serial 同用")
			}
			if len(args) < 2 || len(args) > 3 {
				return fmt.Errorf("需要:目标(IP 或 DevID/MAC)+ 起始地址 + 可选数量")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			addr, err := parseUint16(args[1], "addr")
			if err != nil {
				return exitErr(ExitUsage, err)
			}
			count := uint16(1)
			if len(args) == 3 {
				count, err = parseUint16(args[2], "count")
				if err != nil {
					return exitErr(ExitUsage, err)
				}
			}
			readKind, err := parseReadKind(opt.kind)
			if err != nil {
				return exitErr(ExitUsage, err)
			}
			if err := validateRegRange(addr, int(count), readKind.maxCount); err != nil {
				return exitErr(ExitUsage, err)
			}
			unit, err := parseUnit(opt.unit)
			if err != nil {
				return exitErr(ExitUsage, err)
			}
			host := args[0]
			return withHost(cmd, g, host, func(ep *device.Endpoint) error {
				p, err := readRegParam(cmd, g, ep, host)
				if err != nil {
					return err
				}
				path, err := resolveRegDataPath(&p, opt)
				if err != nil {
					return err
				}
				emitRegWarnings(cmd, g, path.Warnings)
				client, err := modbus.DialTCP(cmd.Context(), path.Address, path.Mode, g.timeout)
				if err != nil {
					return err
				}
				defer client.Close()
				var values []uint16
				if readKind.bits {
					bits, err := client.ReadBits(unit, readKind.fn, addr, count)
					if err != nil {
						return err
					}
					values = bitValues(bits)
				} else {
					values, err = client.ReadRegisters(unit, readKind.fn, addr, count)
					if err != nil {
						return err
					}
				}
				if err != nil {
					return err
				}
				return renderRegRead(cmd, g, host, path, unit, readKind.kind, addr, values)
			})
		},
	}
	cmd.Flags().StringVar(&opt.kind, "kind", "holding", "读取类型:holding|input|coil|discrete")
	return cmd
}

func newRegWriteCmd(g *globalFlags, opt *regOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "write <target> <addr> <value> [value...]",
		Short: "写入 Modbus holding register 或单个 coil",
		Example: `  zlan reg write 192.168.15.42 0x0002 11 --unit 1
  zlan reg write 192.168.15.42 0x0010 0x0001 0x0002 --unit 11
  zlan reg write 192.168.15.47 0x0001 off --kind coil --unit 133`,
		Args: func(_ *cobra.Command, args []string) error {
			if g.serial != "" {
				return fmt.Errorf("reg 通过 ZLAN 网络数据通道读写 Modbus 数据点,不能与 --serial 同用")
			}
			if len(args) < 3 {
				return fmt.Errorf("需要:目标(IP 或 DevID/MAC)+ 起始寄存器地址 + 至少一个值")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			addr, err := parseUint16(args[1], "addr")
			if err != nil {
				return exitErr(ExitUsage, err)
			}
			kind, coil, err := parseWriteKind(opt.kind)
			if err != nil {
				return exitErr(ExitUsage, err)
			}
			var (
				coilValue bool
				values    []uint16
			)
			if coil {
				if len(args) != 3 {
					return exitErr(ExitUsage, fmt.Errorf("--kind coil 一次只能写一个值"))
				}
				coilValue, err = parseCoilValue(args[2])
				if err != nil {
					return exitErr(ExitUsage, err)
				}
				if coilValue {
					values = []uint16{1}
				} else {
					values = []uint16{0}
				}
			} else {
				values, err = parseRegValues(args[2:])
				if err != nil {
					return exitErr(ExitUsage, err)
				}
				if err := validateRegRange(addr, len(values), 123); err != nil {
					return exitErr(ExitUsage, err)
				}
			}
			unit, err := parseUnit(opt.unit)
			if err != nil {
				return exitErr(ExitUsage, err)
			}
			host := args[0]
			return withHost(cmd, g, host, func(ep *device.Endpoint) error {
				p, err := readRegParam(cmd, g, ep, host)
				if err != nil {
					return err
				}
				path, err := resolveRegDataPath(&p, opt)
				if err != nil {
					return err
				}
				emitRegWarnings(cmd, g, path.Warnings)
				client, err := modbus.DialTCP(cmd.Context(), path.Address, path.Mode, g.timeout)
				if err != nil {
					return err
				}
				defer client.Close()
				if coil {
					if err := client.WriteCoil(unit, addr, coilValue); err != nil {
						return err
					}
				} else {
					if err := client.WriteRegisters(unit, addr, values); err != nil {
						return err
					}
				}
				return renderRegWrite(cmd, g, host, path, unit, kind, addr, values)
			})
		},
	}
	cmd.Flags().StringVar(&opt.kind, "kind", "holding", "写入类型:holding|coil")
	return cmd
}

func readRegParam(cmd *cobra.Command, g *globalFlags, ep *device.Endpoint, host string) (protocol.Param, error) {
	p, err := readParam(ep)
	if err == nil {
		return p, nil
	}
	ip := net.ParseIP(host)
	if g.serial != "" || ip == nil {
		return p, err
	}
	devs, derr := device.Discover(cmd.Context(), g.timeout)
	if derr != nil {
		return p, err
	}
	for i := range devs {
		dp := &devs[i].Param
		if get(dp, "local_ip") == host || devs[i].Addr.IP.Equal(ip) {
			return devs[i].Param, nil
		}
	}
	return p, err
}

func resolveRegDataPath(p *protocol.Param, opt *regOptions) (regDataPath, error) {
	workMode := get(p, "work_mode")
	appProto := get(p, "app_proto")
	if workMode != "tcp-server" {
		return regDataPath{}, fmt.Errorf("设备当前 work_mode=%s,CLI 不能主动打开寄存器数据通道;请先改为 tcp-server 或在 tcp-client/udp 对端操作", workMode)
	}

	host := opt.dataHost
	if host == "" {
		host = get(p, "local_ip")
	}
	if strings.TrimSpace(host) == "" {
		return regDataPath{}, fmt.Errorf("数据通道主机不能为空")
	}

	port := opt.dataPort
	if port == 0 {
		v, err := strconv.Atoi(get(p, "local_port"))
		if err != nil {
			return regDataPath{}, fmt.Errorf("设备 local_port 无法解析:%w", err)
		}
		port = v
	}
	if port <= 0 || port > 65535 {
		return regDataPath{}, fmt.Errorf("数据通道端口需在 1..65535")
	}

	mode, warnings, err := resolveRegMode(appProto, opt.mode)
	if err != nil {
		return regDataPath{}, err
	}
	if p.Connected() {
		warnings = append(warnings, "设备 status.connected=1;如果固件只允许单 TCP 连接,本次寄存器访问可能被已有连接占用")
	}
	return regDataPath{
		Address:  net.JoinHostPort(host, strconv.Itoa(port)),
		Mode:     mode,
		WorkMode: workMode,
		AppProto: appProto,
		Warnings: warnings,
	}, nil
}

func resolveRegMode(appProto, want string) (modbus.Mode, []string, error) {
	switch want {
	case "", "auto":
		switch appProto {
		case "modbus":
			return modbus.ModeTCP, nil, nil
		case "transparent":
			return modbus.ModeRTUOverTCP, nil, nil
		default:
			return "", nil, fmt.Errorf("设备 app_proto=%s,无法自动选择寄存器协议", appProto)
		}
	case string(modbus.ModeTCP):
		var warnings []string
		if appProto != "modbus" {
			warnings = append(warnings, fmt.Sprintf("--mode=modbus-tcp 覆盖了设备 app_proto=%s", appProto))
		}
		return modbus.ModeTCP, warnings, nil
	case string(modbus.ModeRTUOverTCP):
		var warnings []string
		if appProto != "transparent" {
			warnings = append(warnings, fmt.Sprintf("--mode=rtu-over-tcp 覆盖了设备 app_proto=%s", appProto))
		}
		return modbus.ModeRTUOverTCP, warnings, nil
	default:
		return "", nil, fmt.Errorf("--mode 需为 auto|modbus-tcp|rtu-over-tcp")
	}
}

func parseUnit(v uint) (byte, error) {
	if v == 0 || v > 247 {
		return 0, fmt.Errorf("--unit 需在 1..247")
	}
	return byte(v), nil
}

func parseUint16(raw, label string) (uint16, error) {
	v, err := strconv.ParseUint(raw, 0, 16)
	if err != nil {
		return 0, fmt.Errorf("%s 需为 0..65535(支持 0x 前缀):%s", label, raw)
	}
	return uint16(v), nil
}

func parseRegValues(args []string) ([]uint16, error) {
	var values []uint16
	for _, arg := range args {
		for _, part := range strings.Split(arg, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			v, err := parseUint16(part, "value")
			if err != nil {
				return nil, err
			}
			values = append(values, v)
		}
	}
	if len(values) == 0 {
		return nil, fmt.Errorf("至少给一个寄存器值")
	}
	return values, nil
}

func validateRegRange(addr uint16, count int, maxCount int) error {
	if count <= 0 || count > maxCount {
		return fmt.Errorf("寄存器数量需在 1..%d", maxCount)
	}
	if int(addr)+count-1 > 0xffff {
		return fmt.Errorf("寄存器范围超出 0xffff")
	}
	return nil
}

type readKindSpec struct {
	fn       byte
	kind     string
	bits     bool
	maxCount int
}

func parseReadKind(raw string) (readKindSpec, error) {
	switch raw {
	case "", "holding", "hold", "hr":
		return readKindSpec{fn: modbus.FuncReadHoldingRegisters, kind: "holding", maxCount: 125}, nil
	case "input", "ir":
		return readKindSpec{fn: modbus.FuncReadInputRegisters, kind: "input", maxCount: 125}, nil
	case "coil", "coils", "c":
		return readKindSpec{fn: modbus.FuncReadCoils, kind: "coil", bits: true, maxCount: 2000}, nil
	case "discrete", "discrete-input", "discrete-inputs", "di":
		return readKindSpec{fn: modbus.FuncReadDiscreteInputs, kind: "discrete", bits: true, maxCount: 2000}, nil
	default:
		return readKindSpec{}, fmt.Errorf("--kind 需为 holding|input|coil|discrete")
	}
}

func parseWriteKind(raw string) (kind string, coil bool, err error) {
	switch raw {
	case "", "holding", "hold", "hr":
		return "holding", false, nil
	case "coil", "coils", "c":
		return "coil", true, nil
	default:
		return "", false, fmt.Errorf("--kind 需为 holding|coil")
	}
}

func parseCoilValue(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "t", "on", "yes", "y":
		return true, nil
	case "0", "false", "f", "off", "no", "n":
		return false, nil
	default:
		return false, fmt.Errorf("coil 值需为 on|off|true|false|1|0")
	}
}

func bitValues(bits []bool) []uint16 {
	values := make([]uint16, len(bits))
	for i, bit := range bits {
		if bit {
			values[i] = 1
		}
	}
	return values
}

func emitRegWarnings(cmd *cobra.Command, g *globalFlags, warnings []string) {
	if g.quiet || g.jsonOut {
		return
	}
	for _, w := range warnings {
		cmd.PrintErrln(yellow("警告: " + w))
	}
}

type regValueJSON struct {
	Address uint16 `json:"address"`
	Value   uint16 `json:"value"`
	Hex     string `json:"hex"`
}

type regResultJSON struct {
	SchemaVersion int            `json:"schema_version"`
	Target        string         `json:"target"`
	DataAddress   string         `json:"data_address"`
	Mode          string         `json:"mode"`
	WorkMode      string         `json:"work_mode"`
	AppProto      string         `json:"app_proto"`
	Unit          byte           `json:"unit"`
	Kind          string         `json:"kind,omitempty"`
	Address       uint16         `json:"address"`
	Count         int            `json:"count"`
	Values        []regValueJSON `json:"values"`
	Warnings      []string       `json:"warnings,omitempty"`
}

func renderRegRead(cmd *cobra.Command, g *globalFlags, target string, path regDataPath, unit byte, kind string, addr uint16, values []uint16) error {
	rows := regRows(addr, values)
	if g.jsonOut {
		return writeJSON(cmd.OutOrStdout(), regResultJSON{
			SchemaVersion: jsonSchemaVersion,
			Target:        target,
			DataAddress:   path.Address,
			Mode:          string(path.Mode),
			WorkMode:      path.WorkMode,
			AppProto:      path.AppProto,
			Unit:          unit,
			Kind:          kind,
			Address:       addr,
			Count:         len(values),
			Values:        rows,
			Warnings:      path.Warnings,
		})
	}
	fmt.Fprintf(cmd.OutOrStdout(), "mode=%s data=%s unit=%d kind=%s\n", path.Mode, path.Address, unit, kind)
	return renderRegRows(cmd.OutOrStdout(), rows)
}

func renderRegWrite(cmd *cobra.Command, g *globalFlags, target string, path regDataPath, unit byte, kind string, addr uint16, values []uint16) error {
	rows := regRows(addr, values)
	if g.jsonOut {
		return writeJSON(cmd.OutOrStdout(), regResultJSON{
			SchemaVersion: jsonSchemaVersion,
			Target:        target,
			DataAddress:   path.Address,
			Mode:          string(path.Mode),
			WorkMode:      path.WorkMode,
			AppProto:      path.AppProto,
			Unit:          unit,
			Kind:          kind,
			Address:       addr,
			Count:         len(values),
			Values:        rows,
			Warnings:      path.Warnings,
		})
	}
	fmt.Fprintf(cmd.OutOrStdout(), "wrote %d %s value(s) via %s to %s unit=%d\n", len(values), kind, path.Mode, path.Address, unit)
	return renderRegRows(cmd.OutOrStdout(), rows)
}

func regRows(addr uint16, values []uint16) []regValueJSON {
	rows := make([]regValueJSON, len(values))
	for i, value := range values {
		a := addr + uint16(i)
		rows[i] = regValueJSON{Address: a, Value: value, Hex: fmt.Sprintf("0x%04X", value)}
	}
	return rows
}

func renderRegRows(w interface {
	Write([]byte) (int, error)
}, rows []regValueJSON) error {
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "ADDR\tDEC\tHEX")
	for _, row := range rows {
		fmt.Fprintf(tw, "0x%04X\t%d\t%s\n", row.Address, row.Value, row.Hex)
	}
	return tw.Flush()
}
