package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"zlan/internal/device"
)

func newApplyCmd(g *globalFlags) *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "按 YAML 清单批量配置设备",
		Long: `按声明式 YAML 清单批量配置设备(按 DevID 匹配——IP 改完即变,不能作主键)。

默认 dry-run:只打印每台 before->after,不写入。确认无误后加 --confirm 实际执行。

清单格式:
  - match: {devid: "aa:bb:cc:dd:ee:ff"}
    set:   {local_ip: 10.0.0.11, net_mask: 255.255.255.0}

示例:
  zlan apply -f devices.yaml            # dry-run 预览
  zlan apply -f devices.yaml --confirm  # 实际执行`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if g.serial != "" {
				return exitErr(ExitUsage, fmt.Errorf("apply 是批量网络操作,不能与 --serial 同用"))
			}
			if file == "" {
				return exitErr(ExitUsage, fmt.Errorf("需用 -f 指定 YAML 清单文件"))
			}
			items, err := device.ParseManifest(file)
			if err != nil {
				return exitErr(ExitUsage, err)
			}
			dryRun := !g.confirm
			if dryRun && !g.quiet {
				cmd.PrintErrln(yellow("DRY-RUN:仅预览不写入;确认无误后加 --confirm 执行。"))
			}
			outcomes, err := device.Apply(cmd.Context(), items, g.timeout, g.retries, dryRun)
			if err != nil {
				return err
			}
			return renderApply(cmd, g, outcomes, dryRun)
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "YAML 清单文件路径")
	return cmd
}

func renderApply(cmd *cobra.Command, g *globalFlags, outcomes []device.ApplyOutcome, dryRun bool) error {
	out := cmd.OutOrStdout()
	var applied, notfound, failed int

	if g.jsonOut {
		type changeJSON struct {
			DevID   string            `json:"devid"`
			Found   bool              `json:"found"`
			Error   string            `json:"error,omitempty"`
			Changed map[string]string `json:"changed,omitempty"`
		}
		arr := make([]changeJSON, 0, len(outcomes))
		for _, oc := range outcomes {
			cj := changeJSON{DevID: oc.DevID, Found: oc.Found}
			switch {
			case !oc.Found:
				notfound++
			case oc.Err != nil:
				failed++
				cj.Error = oc.Err.Error()
			default:
				applied++
				cj.Changed = make(map[string]string, len(oc.Result.Changed))
				for _, f := range oc.Result.Changed {
					a, _ := oc.Result.After.GetField(f)
					cj.Changed[f] = a
				}
			}
			arr = append(arr, cj)
		}
		if err := writeJSON(out, arr); err != nil {
			return err
		}
	} else {
		for _, oc := range outcomes {
			switch {
			case !oc.Found:
				notfound++
				fmt.Fprintf(out, "%s: 未在局域网发现\n", oc.DevID)
			case oc.Err != nil:
				failed++
				fmt.Fprintf(out, "%s: 错误:%v\n", oc.DevID, oc.Err)
			default:
				applied++
				fmt.Fprintf(out, "%s:\n", oc.DevID)
				for _, f := range oc.Result.Changed {
					b, _ := oc.Result.Before.GetField(f)
					a, _ := oc.Result.After.GetField(f)
					fmt.Fprintf(out, "  %s: %s -> %s\n", f, b, a)
				}
			}
		}
	}

	verb := "已应用"
	if dryRun {
		verb = "将应用(dry-run)"
	}
	cmd.PrintErrf("%s %d 台,未发现 %d,失败 %d\n", verb, applied, notfound, failed)
	if failed > 0 || notfound > 0 {
		return exitErr(ExitPartial, fmt.Errorf("批量部分未完成:失败 %d,未发现 %d", failed, notfound))
	}
	return nil
}
