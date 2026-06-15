package cli

import (
	"bytes"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"zlan/internal/device"
)

func newExportCmd(g *globalFlags) *cobra.Command {
	var output string
	cmd := &cobra.Command{
		Use:   "export [target]",
		Short: "导出设备配置文件",
		Long: `读取设备完整参数并导出为可离线携带的配置文件。

导出文件里的 param_hex 是 import 使用的权威数据;fields 只是便于人工查看的快照。

示例:
  zlan export 192.168.1.200 -o zlan-200.yaml
  zlan export --serial /dev/cu.usbserial-1410 -o backup.yaml`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return withEndpoint(cmd, g, args, func(ep *device.Endpoint) error {
				p, err := readParam(ep)
				if err != nil {
					return err
				}
				cfg := device.NewExportedConfig(p)
				if err := writeExportConfig(cmd, g, output, cfg); err != nil {
					return err
				}
				if output != "" && output != "-" && !g.quiet {
					cmd.PrintErrf("已导出配置到 %s\n", output)
				}
				return nil
			})
		},
	}
	cmd.Args = targetArgs(g)
	cmd.Flags().StringVarP(&output, "output", "o", "", "输出文件路径(默认 stdout; '-' 也表示 stdout)")
	return cmd
}

func newImportCmd(g *globalFlags) *cobra.Command {
	var file string
	var exclude []string
	cmd := &cobra.Command{
		Use:   "import [target] [field=value]...",
		Short: "从配置文件导入到设备",
		Long: `从 zlan export 生成的配置文件导入到 target。

默认 dry-run:只打印 target 将发生的 before->after,不写入。确认无误后加 --confirm 执行。
import 保留 target 自己的 DevID、状态、固件版本和能力位;可用 --exclude 排除字段,或用尾随 field=value 覆盖导入值。

示例:
  zlan import 192.168.1.201 -f zlan-200.yaml
  zlan import 192.168.1.201 -f zlan-200.yaml local_ip=192.168.1.201 --confirm
  zlan import 192.168.1.201 -f zlan-200.yaml --exclude local_ip --confirm
  zlan import --serial /dev/cu.usbserial-1410 -f backup.yaml --confirm`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if file == "" {
				return exitErr(ExitUsage, fmt.Errorf("需用 -f 指定 export 配置文件"))
			}
			host, overrides, err := parseImportArgs(g, args)
			if err != nil {
				return exitErr(ExitUsage, err)
			}
			data, err := readConfigInput(cmd, file)
			if err != nil {
				return err
			}
			source, _, err := device.ParseExportedConfig(data)
			if err != nil {
				return exitErr(ExitUsage, err)
			}
			opt := device.CopyOptions{
				Overrides:       overrides,
				Exclude:         exclude,
				AllowSameDevice: true,
			}
			dryRun := !g.confirm
			if dryRun && !g.quiet {
				cmd.PrintErrln(yellow("DRY-RUN:仅预览不写入;确认无误后加 --confirm 执行。"))
			}

			return withHost(cmd, g, host, func(ep *device.Endpoint) error {
				if dryRun {
					target, err := readParam(ep)
					if err != nil {
						return err
					}
					res, err := device.PlanCopy(source, target, opt)
					if err != nil {
						return err
					}
					return renderCopyResult(cmd, g, res, true)
				}
				res, err := device.CopyConfig(ep.Conn, ep.Snapshot, source, opt)
				if err != nil {
					return err
				}
				return renderCopyResult(cmd, g, res, false)
			})
		},
	}
	cmd.Flags().StringVarP(&file, "file", "f", "", "zlan export 生成的配置文件路径('-' 表示 stdin)")
	cmd.Flags().StringSliceVar(&exclude, "exclude", nil, "不导入指定字段/位域(可重复或逗号分隔)")
	return cmd
}

func parseImportArgs(g *globalFlags, args []string) (host string, overrides map[string]string, err error) {
	overrideArgs := args
	if g.serial == "" {
		if len(args) < 1 {
			return "", nil, fmt.Errorf("需要目标设备(IP 或 DevID/MAC),或用 --serial 走串口")
		}
		host = args[0]
		overrideArgs = args[1:]
	}
	overrides, err = parseAssignments(overrideArgs)
	if err != nil {
		return "", nil, err
	}
	return host, overrides, nil
}

func writeExportConfig(cmd *cobra.Command, g *globalFlags, output string, cfg device.ExportedConfig) error {
	data, err := marshalExportConfig(cfg, g.jsonOut)
	if err != nil {
		return err
	}
	if output == "" || output == "-" {
		_, err = cmd.OutOrStdout().Write(data)
		return err
	}
	if err := os.WriteFile(output, data, 0o644); err != nil {
		return fmt.Errorf("写入配置文件失败: %w", err)
	}
	return nil
}

func marshalExportConfig(cfg device.ExportedConfig, asJSON bool) ([]byte, error) {
	var buf bytes.Buffer
	if asJSON {
		if err := writeJSON(&buf, cfg); err != nil {
			return nil, err
		}
		return buf.Bytes(), nil
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return nil, fmt.Errorf("生成 YAML 失败: %w", err)
	}
	return data, nil
}

func readConfigInput(cmd *cobra.Command, path string) ([]byte, error) {
	if path == "-" {
		data, err := io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return nil, fmt.Errorf("读取 stdin 失败: %w", err)
		}
		return data, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	return data, nil
}
