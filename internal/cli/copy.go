package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"zlan/internal/device"
	"zlan/internal/protocol"
)

func newCopyCmd(g *globalFlags) *cobra.Command {
	var exclude []string
	cmd := &cobra.Command{
		Use:     "copy <source> <target> [field=value]...",
		Aliases: []string{"clone", "cp"},
		Short:   "复制一台设备的配置到另一台",
		Long: `复制 source 的可写配置到 target,保留 target 自己的 DevID、状态、固件版本和能力位。

默认 dry-run:只打印 target 将发生的 before->after,不写入。确认无误后加 --confirm 执行。
如需避免 IP 冲突,可在同一命令末尾覆盖目标字段,例如 local_ip=192.168.1.201。

示例:
  zlan copy 192.168.1.200 192.168.1.201
  zlan copy 5a:4c:6f:73:cc:d6 5a:4c:6f:73:cc:d7 local_ip=192.168.1.201 --confirm
  zlan copy 192.168.1.200 192.168.1.201 --exclude local_ip --confirm`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if g.serial != "" {
				return exitErr(ExitUsage, fmt.Errorf("copy 需要同时访问源和目标两个网络设备,不能与 --serial 同用"))
			}
			sourceHost, targetHost, overrides, err := parseCopyArgs(args)
			if err != nil {
				return exitErr(ExitUsage, err)
			}
			opt := device.CopyOptions{Overrides: overrides, Exclude: exclude}
			dryRun := !g.confirm
			if dryRun && !g.quiet {
				cmd.PrintErrln(yellow("DRY-RUN:仅预览不写入;确认无误后加 --confirm 执行。"))
			}

			sourceEp, err := g.target(sourceHost).Open(cmd.Context())
			if err != nil {
				return fmt.Errorf("读取源设备失败: %w", err)
			}
			source, err := readParam(sourceEp)
			_ = sourceEp.Conn.Close()
			if err != nil {
				return fmt.Errorf("读取源设备失败: %w", err)
			}

			targetEp, err := g.target(targetHost).Open(cmd.Context())
			if err != nil {
				return fmt.Errorf("打开目标设备失败: %w", err)
			}
			defer targetEp.Conn.Close()

			if dryRun {
				target, err := readParam(targetEp)
				if err != nil {
					return fmt.Errorf("读取目标设备失败: %w", err)
				}
				res, err := device.PlanCopy(source, target, opt)
				if err != nil {
					return err
				}
				return renderCopyResult(cmd, g, res, true)
			}

			res, err := device.CopyConfig(targetEp.Conn, targetEp.Snapshot, source, opt)
			if err != nil {
				return err
			}
			return renderCopyResult(cmd, g, res, false)
		},
	}
	cmd.Flags().StringSliceVar(&exclude, "exclude", nil, "不复制指定字段/位域(可重复或逗号分隔)")
	return cmd
}

func parseCopyArgs(args []string) (source, target string, overrides map[string]string, err error) {
	if len(args) < 2 {
		return "", "", nil, fmt.Errorf("需要 source 和 target 两个设备目标(IP 或 DevID/MAC)")
	}
	overrides, err = parseAssignments(args[2:])
	if err != nil {
		return "", "", nil, err
	}
	return args[0], args[1], overrides, nil
}

func parseAssignments(args []string) (map[string]string, error) {
	kvs := make(map[string]string, len(args))
	for _, a := range args {
		k, v, ok := strings.Cut(a, "=")
		if !ok || k == "" {
			return nil, fmt.Errorf("参数格式应为 field=value:%q", a)
		}
		if _, ok := protocol.FieldByName(k); !ok {
			if _, ok := protocol.BitFieldByName(k); !ok {
				return nil, fmt.Errorf("未知字段:%s(用 zlan get --list-fields 查看)", k)
			}
		}
		kvs[k] = v
	}
	return kvs, nil
}

func renderCopyResult(cmd *cobra.Command, g *globalFlags, res *device.SetResult, dryRun bool) error {
	if !dryRun {
		return renderSetResult(cmd, g, res)
	}
	changes := make(map[string]map[string]string, len(res.Changed))
	for _, f := range res.Changed {
		b, _ := res.Before.GetField(f)
		a, _ := res.After.GetField(f)
		changes[f] = map[string]string{"before": b, "after": a}
	}
	if g.jsonOut {
		return writeJSON(cmd.OutOrStdout(), map[string]any{
			"schema_version": jsonSchemaVersion,
			"dry_run":        true,
			"changed":        changes,
			"network_field":  res.NetworkField,
		})
	}
	out := cmd.OutOrStdout()
	if len(res.Changed) == 0 {
		fmt.Fprintln(out, "无变化")
		return nil
	}
	for _, f := range res.Changed {
		c := changes[f]
		fmt.Fprintf(out, "%s: %s -> %s\n", f, c["before"], c["after"])
	}
	if res.NetworkField {
		cmd.PrintErrln(yellow("预览包含网络参数变更;执行后目标设备会重启,并可能切换网段/失联。"))
	}
	return nil
}
