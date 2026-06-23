package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"text/tabwriter"
	"unicode"

	"golang.org/x/term"
	"golang.org/x/text/width"

	"zlan/internal/protocol"
	"zlan/internal/transport"
)

// jsonSchemaVersion 标识 --json 输出契约版本,便于消费方做兼容。
const jsonSchemaVersion = 1

// 颜色门:NO_COLOR / TERM=dumb / 非 TTY / --no-color 任一即关闭(启动判一次)。
var useColor = decideColor()

func decideColor() bool {
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	return term.IsTerminal(int(os.Stdout.Fd()))
}

func disableColor() { useColor = false }

func colorize(s, code string) string {
	if !useColor {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func green(s string) string  { return colorize(s, "32") }
func yellow(s string) string { return colorize(s, "33") }
func bold(s string) string   { return colorize(s, "1") }

func writeJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

type deviceJSON struct {
	DevID     string `json:"devid"`
	Name      string `json:"name"`
	IP        string `json:"ip"`
	Mode      string `json:"mode"`
	Baud      string `json:"baud"`
	Ver       string `json:"ver"`
	Connected bool   `json:"connected"`
	Addr      string `json:"addr"`
}

// renderDevices 输出发现到的设备列表(表格或 JSON)。
func renderDevices(w io.Writer, devs []transport.Device, asJSON bool) error {
	if asJSON {
		out := make([]deviceJSON, 0, len(devs))
		for i := range devs {
			p := &devs[i].Param
			out = append(out, deviceJSON{
				DevID:     get(p, "devid"),
				Name:      get(p, "dev_name"),
				IP:        get(p, "local_ip"),
				Mode:      get(p, "work_mode"),
				Baud:      get(p, "baud"),
				Ver:       get(p, "ver"),
				Connected: p.Connected(),
				Addr:      devs[i].Addr.String(),
			})
		}
		return writeJSON(w, out)
	}
	rows := make([][]string, 0, len(devs))
	for i := range devs {
		p := &devs[i].Param
		rows = append(rows, []string{
			get(p, "devid"), get(p, "dev_name"), get(p, "local_ip"),
			get(p, "work_mode"), get(p, "baud"), get(p, "ver"), statusLabel(p),
		})
	}
	return writeAlignedTable(w, []tableColumn{
		{header: "DEVID"},
		{header: "NAME"},
		{header: "IP"},
		{header: "MODE"},
		{header: "BAUD", align: alignRight},
		{header: "VER"},
		{header: "STATUS"},
	}, rows)
}

// renderParam 输出单台设备的完整参数(分组表格或 JSON)。
func renderParam(w io.Writer, p *protocol.Param, asJSON bool) error {
	tlvs, hasTLVs := decodedUserParamTLVs(p)
	wifi, hasWiFi := protocol.WiFiConfig{}, false
	counters, hasCounters := protocol.UserParamCounters{}, false
	if hasTLVs {
		wifi, hasWiFi = decodedWiFiFromTLVs(tlvs)
		counters, hasCounters = decodedUserParamCounters(tlvs)
	}
	rawTLVs := unhandledUserParamTLVs(tlvs, hasWiFi, hasCounters)

	if asJSON {
		m := make(map[string]string, len(protocol.Fields()))
		for _, f := range protocol.Fields() {
			m[f.Name] = get(p, f.Name)
		}
		out := map[string]any{"schema_version": jsonSchemaVersion, "fields": m}
		if hasCounters {
			out["serial_counters"] = serialCountersJSON(counters)
		}
		if hasWiFi {
			out["wifi"] = wifiJSON(wifi, false)
		}
		if len(rawTLVs) > 0 {
			out["user_param_tlvs"] = userParamTLVsJSON(rawTLVs)
		}
		return writeJSON(w, out)
	}
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	for _, grp := range []string{"network", "serial", "behavior", "identity", "advanced"} {
		fmt.Fprintln(tw, bold("["+grp+"]"))
		for _, f := range protocol.Fields() {
			if f.Group != grp {
				continue
			}
			ro := ""
			if f.ReadOnly {
				ro = yellow(" (只读)")
			}
			fmt.Fprintf(tw, "  %s\t%s\t%s%s\n", f.Name, get(p, f.Name), f.Desc, ro)
		}
		if grp == "serial" && hasCounters {
			if counters.HasSerialTxBytes {
				fmt.Fprintf(tw, "  serial_tx_bytes\t%d\t串口发送字节数(Rev.4 TLV)\n", counters.SerialTxBytes)
			}
			if counters.HasSerialRxBytes {
				fmt.Fprintf(tw, "  serial_rx_bytes\t%d\t串口接收字节数(Rev.4 TLV)\n", counters.SerialRxBytes)
			}
		}
	}
	if hasWiFi {
		fmt.Fprintln(tw, bold("[wifi]"))
		for _, row := range []struct{ name, desc string }{
			{"ssid", "WiFi SSID"},
			{"mode", "AP/STA 模式"},
			{"crypt", "加密方式"},
			{"channel", "WiFi 信道"},
			{"dhcp_server", "DHCP Server"},
			{"bridge", "以太网/WiFi 桥接"},
			{"key", "WiFi 密码(默认脱敏)"},
		} {
			fmt.Fprintf(tw, "  %s\t%s\t%s\n", row.name, wifiValue(wifi, row.name, false), row.desc)
		}
	}
	if len(rawTLVs) > 0 {
		fmt.Fprintln(tw, bold("[user_param_tlv]"))
		for i, rec := range rawTLVs {
			fmt.Fprintf(tw, "  tlv[%d]\t%s\tRev.4 user_param TLV\n", i, userTLVSummary(rec))
		}
	}
	return tw.Flush()
}

func decodedWiFi(p *protocol.Param) (protocol.WiFiConfig, bool) {
	tlvs, ok := decodedUserParamTLVs(p)
	if !ok {
		return protocol.WiFiConfig{}, false
	}
	return decodedWiFiFromTLVs(tlvs)
}

func decodedWiFiFromTLVs(tlvs []protocol.UserTLV) (protocol.WiFiConfig, bool) {
	cfg, err := protocol.WiFiConfigFromTLVs(tlvs)
	if err != nil {
		return protocol.WiFiConfig{}, false
	}
	return cfg, cfg.HasSSID || cfg.HasKey || cfg.HasChannel || cfg.HasModeCrypt
}

func decodedUserParamTLVs(p *protocol.Param) ([]protocol.UserTLV, bool) {
	tlvs, err := p.UserTLVs()
	if err != nil || len(tlvs) == 0 {
		return nil, false
	}
	return tlvs, true
}

func decodedUserParamCounters(tlvs []protocol.UserTLV) (protocol.UserParamCounters, bool) {
	counters, err := protocol.UserParamCountersFromTLVs(tlvs)
	if err != nil {
		return protocol.UserParamCounters{}, false
	}
	return counters, counters.HasSerialTxBytes || counters.HasSerialRxBytes
}

func serialCountersJSON(counters protocol.UserParamCounters) map[string]any {
	out := map[string]any{
		"tx_bytes": nil,
		"rx_bytes": nil,
	}
	if counters.HasSerialTxBytes {
		out["tx_bytes"] = counters.SerialTxBytes
	}
	if counters.HasSerialRxBytes {
		out["rx_bytes"] = counters.SerialRxBytes
	}
	return out
}

func unhandledUserParamTLVs(tlvs []protocol.UserTLV, wifiHandled, countersHandled bool) []protocol.UserTLV {
	out := make([]protocol.UserTLV, 0, len(tlvs))
	for _, rec := range tlvs {
		if wifiHandled && isWiFiTLV(rec.Type) {
			continue
		}
		if countersHandled && isSerialCounterTLV(rec.Type) {
			continue
		}
		out = append(out, rec)
	}
	return out
}

func isWiFiTLV(typ byte) bool {
	switch typ {
	case protocol.UserTLVWiFiSSID, protocol.UserTLVWiFiChannel, protocol.UserTLVWiFiModeCrypt, protocol.UserTLVWiFiPassword:
		return true
	default:
		return false
	}
}

func isSerialCounterTLV(typ byte) bool {
	return typ == protocol.UserTLVSerialTxBytes || typ == protocol.UserTLVSerialRxBytes
}

func userParamTLVsJSON(tlvs []protocol.UserTLV) map[string]any {
	out := map[string]any{
		"records": userTLVRecordsJSON(tlvs),
	}
	return out
}

func userTLVRecordsJSON(tlvs []protocol.UserTLV) []map[string]any {
	out := make([]map[string]any, 0, len(tlvs))
	for _, rec := range tlvs {
		row := map[string]any{
			"type":      int(rec.Type),
			"name":      protocol.UserTLVName(rec.Type),
			"length":    len(rec.Value),
			"value_hex": userTLVValueHex(rec),
		}
		if decoded, ok := userTLVDecoded(rec); ok {
			row["decoded"] = decoded
		}
		if rec.Type == protocol.UserTLVWiFiPassword {
			row["redacted"] = true
		}
		out = append(out, row)
	}
	return out
}

func userTLVSummary(rec protocol.UserTLV) string {
	parts := []string{
		"type=" + strconv.Itoa(int(rec.Type)),
		"name=" + protocol.UserTLVName(rec.Type),
		"len=" + strconv.Itoa(len(rec.Value)),
		"value=" + userTLVValueText(rec),
	}
	if decoded, ok := userTLVDecoded(rec); ok {
		parts = append(parts, "decoded="+fmt.Sprint(decoded))
	}
	return strings.Join(parts, " ")
}

func userTLVValueHex(rec protocol.UserTLV) any {
	if rec.Type == protocol.UserTLVWiFiPassword {
		return nil
	}
	return hex.EncodeToString(rec.Value)
}

func userTLVValueText(rec protocol.UserTLV) string {
	if rec.Type == protocol.UserTLVWiFiPassword {
		return "<redacted>"
	}
	return hex.EncodeToString(rec.Value)
}

func userTLVDecoded(rec protocol.UserTLV) (any, bool) {
	switch rec.Type {
	case protocol.UserTLVSerialTxBytes, protocol.UserTLVSerialRxBytes:
		counters, err := protocol.UserParamCountersFromTLVs([]protocol.UserTLV{rec})
		if err != nil {
			return nil, false
		}
		if counters.HasSerialTxBytes {
			return counters.SerialTxBytes, true
		}
		return counters.SerialRxBytes, true
	case protocol.UserTLVWiFiSSID:
		return string(rec.Value), true
	}
	return nil, false
}

func statusLabel(p *protocol.Param) string {
	if p.Connected() {
		return green("connected")
	}
	return "idle"
}

type tableAlign uint8

const alignRight tableAlign = 1

type tableColumn struct {
	header string
	align  tableAlign
}

func writeAlignedTable(w io.Writer, cols []tableColumn, rows [][]string) error {
	widths := make([]int, len(cols))
	for i, col := range cols {
		widths[i] = displayWidth(col.header)
	}
	for _, row := range rows {
		for i := range cols {
			cell := ""
			if i < len(row) {
				cell = row[i]
			}
			widths[i] = max(widths[i], displayWidth(cell))
		}
	}

	var b strings.Builder
	writeTableRow(&b, cols, widths, nil, true)
	for _, row := range rows {
		writeTableRow(&b, cols, widths, row, false)
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func writeTableRow(b *strings.Builder, cols []tableColumn, widths []int, row []string, header bool) {
	for i, col := range cols {
		cell := col.header
		if !header {
			cell = ""
			if i < len(row) {
				cell = row[i]
			}
		}
		if i == len(cols)-1 {
			b.WriteString(cell)
			break
		}
		pad := widths[i] - displayWidth(cell)
		if col.align == alignRight && !header {
			b.WriteString(strings.Repeat(" ", pad))
			b.WriteString(cell)
		} else {
			b.WriteString(cell)
			b.WriteString(strings.Repeat(" ", pad))
		}
		if i != len(cols)-1 {
			b.WriteString("  ")
		}
	}
	b.WriteByte('\n')
}

func displayWidth(s string) int {
	n := 0
	ansiState := 0
	for _, r := range s {
		switch ansiState {
		case 1:
			if r == '[' {
				ansiState = 2
			} else {
				ansiState = 0
			}
			continue
		case 2:
			if r >= '@' && r <= '~' {
				ansiState = 0
			}
			continue
		}
		if r == '\x1b' {
			ansiState = 1
			continue
		}
		if r == '\t' {
			n += 4
			continue
		}
		if r < ' ' || r == 0x7f || unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || r == '\u200d' {
			continue
		}
		switch width.LookupRune(r).Kind() {
		case width.EastAsianWide, width.EastAsianFullwidth:
			n += 2
		default:
			n++
		}
	}
	return n
}

// get 是 GetField 的便捷封装(展示场景忽略未知字段错误)。
func get(p *protocol.Param, name string) string {
	v, _ := p.GetField(name)
	return v
}
