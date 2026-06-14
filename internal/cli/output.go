package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"text/tabwriter"

	"golang.org/x/term"

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
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "DEVID\tNAME\tIP\tMODE\tBAUD\tVER\tSTATUS")
	for i := range devs {
		p := &devs[i].Param
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			get(p, "devid"), get(p, "dev_name"), get(p, "local_ip"),
			get(p, "work_mode"), get(p, "baud"), get(p, "ver"), statusLabel(p))
	}
	return tw.Flush()
}

// renderParam 输出单台设备的完整参数(分组表格或 JSON)。
func renderParam(w io.Writer, p *protocol.Param, asJSON bool) error {
	if asJSON {
		m := make(map[string]string, len(protocol.Fields()))
		for _, f := range protocol.Fields() {
			m[f.Name] = get(p, f.Name)
		}
		return writeJSON(w, map[string]any{"schema_version": jsonSchemaVersion, "fields": m})
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
	}
	return tw.Flush()
}

func statusLabel(p *protocol.Param) string {
	if p.Connected() {
		return green("connected")
	}
	return "idle"
}

// get 是 GetField 的便捷封装(展示场景忽略未知字段错误)。
func get(p *protocol.Param, name string) string {
	v, _ := p.GetField(name)
	return v
}
