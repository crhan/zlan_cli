package cli

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"zlan/internal/device"
	"zlan/internal/protocol"
)

// renderSetResult 输出改动 diff(stdout)与生效状态(stderr)。
func renderSetResult(cmd *cobra.Command, g *globalFlags, res *device.SetResult) error {
	if g.jsonOut {
		changes := make(map[string]map[string]string, len(res.Changed))
		for _, f := range res.Changed {
			b, _ := res.Before.GetField(f)
			a, _ := res.After.GetField(f)
			changes[f] = map[string]string{"before": b, "after": a}
		}
		return writeJSON(cmd.OutOrStdout(), map[string]any{
			"schema_version": jsonSchemaVersion,
			"changed":        changes,
			"network_field":  res.NetworkField,
			"verified":       res.Verified,
		})
	}
	out := cmd.OutOrStdout()
	for _, f := range res.Changed {
		b, _ := res.Before.GetField(f)
		a, _ := res.After.GetField(f)
		fmt.Fprintf(out, "%s: %s -> %s\n", f, b, a)
	}
	switch {
	case res.NetworkField:
		cmd.PrintErrln(yellow("已写入,设备保存并重启;改了网络参数,设备可能切换网段。请在对应网段重新 discover/info 确认。"))
	case res.Verified:
		cmd.PrintErrln(green("已生效(读回校验通过)。"))
	default:
		cmd.PrintErrf(yellow("已写入;读回未确认(%v),设备可能正在重启,稍后用 info 确认。\n"), res.VerifyErr)
	}
	return nil
}

// listAllFields 列出所有字段/位域及其类型、可选值,供 set/get 参考。
func listAllFields(w io.Writer, asJSON bool) error {
	type fieldMeta struct {
		Name     string   `json:"name"`
		Group    string   `json:"group"`
		ReadOnly bool     `json:"read_only"`
		Options  []string `json:"options,omitempty"`
		Desc     string   `json:"desc"`
	}
	var metas []fieldMeta
	for _, f := range protocol.Fields() {
		metas = append(metas, fieldMeta{f.Name, f.Group, f.ReadOnly, protocol.FieldOptions(f.Name), f.Desc})
	}
	for _, b := range protocol.BitFields() {
		metas = append(metas, fieldMeta{b.Name, "bit", b.ReadOnly, []string{"0", "1"}, b.Desc})
	}

	if asJSON {
		return writeJSON(w, metas)
	}
	tw := tabwriter.NewWriter(w, 0, 2, 2, ' ', 0)
	fmt.Fprintln(tw, "FIELD\tGROUP\tRW\tOPTIONS\tDESC")
	for _, m := range metas {
		rw := "rw"
		if m.ReadOnly {
			rw = "ro"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", m.Name, m.Group, rw, strings.Join(m.Options, "|"), m.Desc)
	}
	return tw.Flush()
}
