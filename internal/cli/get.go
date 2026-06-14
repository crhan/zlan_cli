package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"zlan/internal/device"
)

func newGetCmd(g *globalFlags) *cobra.Command {
	var listFields bool
	cmd := &cobra.Command{
		Use:   "get [target] <field>",
		Short: "读取单个字段(脚本友好)",
		Long: `读取设备单个字段的值。字段名见 zlan get --list-fields。

示例:
  zlan get 192.168.1.200 local_ip
  zlan get 192.168.1.200 func_en.need_password
  zlan get --list-fields`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if listFields {
				return listAllFields(cmd.OutOrStdout(), g.jsonOut)
			}
			host, field, err := parseGetArgs(g, args)
			if err != nil {
				return exitErr(ExitUsage, err)
			}
			return withHost(cmd, g, host, func(ep *device.Endpoint) error {
				p, err := readParam(ep)
				if err != nil {
					return err
				}
				v, err := p.GetField(field)
				if err != nil {
					return exitErr(ExitUsage, err)
				}
				if g.jsonOut {
					return writeJSON(cmd.OutOrStdout(), map[string]string{field: v})
				}
				fmt.Fprintln(cmd.OutOrStdout(), v)
				return nil
			})
		},
	}
	cmd.Flags().BoolVar(&listFields, "list-fields", false, "列出所有字段及可选值并退出")
	return cmd
}

func parseGetArgs(g *globalFlags, args []string) (host, field string, err error) {
	if g.serial != "" {
		if len(args) != 1 {
			return "", "", fmt.Errorf("串口模式需要一个字段名")
		}
		return "", args[0], nil
	}
	if len(args) != 2 {
		return "", "", fmt.Errorf("需要:目标(IP 或 DevID/MAC)+ 字段名")
	}
	return args[0], args[1], nil
}
