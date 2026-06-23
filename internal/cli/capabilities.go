package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"zlan/internal/device"
	"zlan/internal/protocol"
	"zlan/internal/transport"
)

type capabilityDef struct {
	Field string `json:"field"`
	Name  string `json:"name"`
	Desc  string `json:"desc"`
}

var capabilityDefs = []capabilityDef{
	{Field: "func_sel.web_download", Name: "web_download", Desc: "网页下载"},
	{Field: "func_sel.dns", Name: "dns", Desc: "DNS 域名"},
	{Field: "func_sel.realcom", Name: "realcom", Desc: "REAL_COM 协议"},
	{Field: "func_sel.modbus_tcp_to_rtu", Name: "modbus_tcp_to_rtu", Desc: "Modbus TCP 转 RTU"},
	{Field: "func_sel.serial_param_modify", Name: "serial_param_modify", Desc: "串口修改参数"},
	{Field: "func_sel.dhcp", Name: "dhcp", Desc: "自动获取 IP(DHCP)"},
	{Field: "func_sel.storage_ex", Name: "storage_ex", Desc: "存储扩展 EX"},
	{Field: "func_sel.multi_tcp", Name: "multi_tcp", Desc: "多 TCP 连接"},
	{Field: "func_sel2.io_config", Name: "io_config", Desc: "IO 配置"},
	{Field: "func_sel2.udp_multicast", Name: "udp_multicast", Desc: "UDP 组播"},
	{Field: "func_sel2.multi_target_ip", Name: "multi_target_ip", Desc: "多目标 IP"},
	{Field: "func_sel2.proxy_server", Name: "proxy_server", Desc: "代理服务器"},
	{Field: "func_sel2.snmp", Name: "snmp", Desc: "SNMP"},
	{Field: "func_sel2.p2p", Name: "p2p", Desc: "P2P"},
}

type capabilityJSON struct {
	DevID          string            `json:"devid"`
	Name           string            `json:"name"`
	IP             string            `json:"ip"`
	Ver            string            `json:"ver"`
	FuncSel        string            `json:"func_sel"`
	FuncSel2       string            `json:"func_sel2"`
	Supported      []string          `json:"supported"`
	Capabilities   map[string]bool   `json:"capabilities"`
	UnknownBits    map[string]string `json:"unknown_bits,omitempty"`
	ModelGuess     string            `json:"model_guess"`
	ModelGuessNote string            `json:"model_guess_note"`
}

type capabilityRow struct {
	DevID          string
	Name           string
	IP             string
	Ver            string
	FuncSel        string
	FuncSel2       string
	Supported      []string
	UnknownBits    map[string]string
	ModelGuess     string
	ModelGuessNote string
}

func newCapabilitiesCmd(g *globalFlags) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "capabilities [target]",
		Aliases: []string{"caps", "cap"},
		Short:   "查看设备能力位(func_sel/func_sel2)",
		Long: `查看 ZLAN 参数块中的只读能力位(func_sel/func_sel2)。
不传 target 时 discover 当前可见设备并批量对比;传 target 时读取单台设备。

能力位只能说明设备/固件支持的功能,不能可靠推出具体产品型号。`,
		Example: `  zlan capabilities
  zlan capabilities 192.168.15.47
  zlan capabilities --json`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return fmt.Errorf("最多传一个目标(IP 或 DevID/MAC)")
			}
			if g.serial != "" && len(args) != 0 {
				return fmt.Errorf("串口模式不需要 target")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 && g.serial == "" {
				devs, err := device.Discover(cmd.Context(), g.timeout)
				if err != nil {
					return err
				}
				if len(devs) == 0 {
					if g.jsonOut {
						return writeJSON(cmd.OutOrStdout(), []capabilityJSON{})
					}
					return exitErr(ExitNotFound, fmt.Errorf(
						"未发现任何设备(确认设备上电、与本机同一局域网、防火墙放行 UDP %d)", protocol.MgmtPort))
				}
				return renderCapabilities(cmd.OutOrStdout(), capabilityRowsFromDevices(devs), g.jsonOut)
			}

			return withEndpoint(cmd, g, args, func(ep *device.Endpoint) error {
				p, err := readParam(ep)
				if err != nil {
					return err
				}
				return renderCapabilities(cmd.OutOrStdout(), []capabilityRow{capabilityRowFromParam(&p)}, g.jsonOut)
			})
		},
	}
	return cmd
}

func capabilityRowsFromDevices(devs []transport.Device) []capabilityRow {
	rows := make([]capabilityRow, 0, len(devs))
	for i := range devs {
		rows = append(rows, capabilityRowFromParam(&devs[i].Param))
	}
	return rows
}

func capabilityRowFromParam(p *protocol.Param) capabilityRow {
	row := capabilityRow{
		DevID:          get(p, "devid"),
		Name:           get(p, "dev_name"),
		IP:             get(p, "local_ip"),
		Ver:            get(p, "ver"),
		FuncSel:        get(p, "func_sel"),
		FuncSel2:       get(p, "func_sel2"),
		Supported:      supportedCapabilities(p),
		UnknownBits:    unknownCapabilityBits(p),
		ModelGuess:     "unknown",
		ModelGuessNote: "capabilities are not a reliable product model identifier",
	}
	return row
}

func supportedCapabilities(p *protocol.Param) []string {
	out := make([]string, 0, len(capabilityDefs))
	for _, cap := range capabilityDefs {
		if get(p, cap.Field) == "1" {
			out = append(out, cap.Name)
		}
	}
	return out
}

func unknownCapabilityBits(p *protocol.Param) map[string]string {
	unknown := map[string]string{}
	if bits := unknownBits(get(p, "func_sel"), 0xff); bits != "" {
		unknown["func_sel"] = bits
	}
	if bits := unknownBits(get(p, "func_sel2"), 0x3f); bits != "" {
		unknown["func_sel2"] = bits
	}
	if len(unknown) == 0 {
		return nil
	}
	return unknown
}

func unknownBits(hexByte string, knownMask byte) string {
	var v uint64
	if _, err := fmt.Sscanf(hexByte, "0x%02x", &v); err != nil {
		return ""
	}
	unknown := byte(v) &^ knownMask
	if unknown == 0 {
		return ""
	}
	names := make([]string, 0, 8)
	for i := 0; i < 8; i++ {
		if unknown&(1<<i) != 0 {
			names = append(names, fmt.Sprintf("bit%d", i))
		}
	}
	return strings.Join(names, "|")
}

func renderCapabilities(w interface {
	Write([]byte) (int, error)
}, rows []capabilityRow, asJSON bool) error {
	if asJSON {
		out := make([]capabilityJSON, 0, len(rows))
		for _, row := range rows {
			out = append(out, capabilityJSON{
				DevID:          row.DevID,
				Name:           row.Name,
				IP:             row.IP,
				Ver:            row.Ver,
				FuncSel:        row.FuncSel,
				FuncSel2:       row.FuncSel2,
				Supported:      row.Supported,
				Capabilities:   capabilityMapFromNames(row.Supported),
				UnknownBits:    row.UnknownBits,
				ModelGuess:     row.ModelGuess,
				ModelGuessNote: row.ModelGuessNote,
			})
		}
		return writeJSON(w, out)
	}

	tableRows := make([][]string, 0, len(rows))
	for _, row := range rows {
		tableRows = append(tableRows, []string{
			row.DevID,
			row.Name,
			row.IP,
			row.Ver,
			row.FuncSel,
			row.FuncSel2,
			strings.Join(row.Supported, ","),
			formatUnknownBits(row.UnknownBits),
		})
	}
	return writeAlignedTable(w, []tableColumn{
		{header: "DEVID"},
		{header: "NAME"},
		{header: "IP"},
		{header: "VER"},
		{header: "FUNC"},
		{header: "FUNC2"},
		{header: "CAPABILITIES"},
		{header: "UNKNOWN"},
	}, tableRows)
}

func capabilityMapFromNames(names []string) map[string]bool {
	out := make(map[string]bool, len(capabilityDefs))
	for _, cap := range capabilityDefs {
		out[cap.Name] = false
	}
	for _, name := range names {
		out[name] = true
	}
	return out
}

func formatUnknownBits(bits map[string]string) string {
	if len(bits) == 0 {
		return ""
	}
	parts := make([]string, 0, len(bits))
	for _, key := range []string{"func_sel", "func_sel2"} {
		if v := bits[key]; v != "" {
			parts = append(parts, key+"="+v)
		}
	}
	return strings.Join(parts, ",")
}
