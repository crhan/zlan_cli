package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

// regPollSpec 收敛一次轮询的全部参数,避免 runRegPoll 形参过长。
type regPollSpec struct {
	host     string
	path     regDataPath
	unit     byte
	fn       byte
	kind     string
	addr     uint16
	count    uint16
	interval time.Duration
	times    int // 0 = 无限,Ctrl-C 退出
}

func newRegPollCmd(g *globalFlags, opt *regOptions) *cobra.Command {
	var interval time.Duration
	var times int
	cmd := &cobra.Command{
		Use:   "poll <target> <addr> [count]",
		Short: "在数据通道长连接上定时轮询同一组寄存器",
		Long: `建立一条到 ZLAN 数据通道的 TCP 长连接,按固定间隔在同一连接上反复读取同一组寄存器
并输出。连接因设备 keep_alive 空闲超时断开时自动重连。Ctrl-C 退出。

示例:
  zlan reg poll 192.168.1.200 0x0001 4 --interval 1s --unit 1
  zlan reg poll 192.168.1.200 0x0001 --kind input --json
  zlan reg poll 192.168.1.200 0x0001 2 --times 10`,
		Args: func(_ *cobra.Command, args []string) error {
			if g.serial != "" {
				return fmt.Errorf("reg 通过 ZLAN 网络数据通道读写寄存器,不能与 --serial 同用")
			}
			if len(args) < 2 || len(args) > 3 {
				return fmt.Errorf("需要:目标(IP 或 DevID/MAC)+ 起始寄存器地址 + 可选数量")
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
			if err := validateRegRange(addr, int(count), 125); err != nil {
				return exitErr(ExitUsage, err)
			}
			fn, kind, err := readKind(opt.kind)
			if err != nil {
				return exitErr(ExitUsage, err)
			}
			unit, err := parseUnit(opt.unit)
			if err != nil {
				return exitErr(ExitUsage, err)
			}
			if interval <= 0 {
				return exitErr(ExitUsage, fmt.Errorf("--interval 需为正"))
			}
			host := args[0]
			path, err := resolveRegSessionPath(cmd, g, host, opt)
			if err != nil {
				return err
			}
			emitRegWarnings(cmd, g, path.Warnings)
			sess, err := newRegSession(cmd.Context(), path, g.timeout)
			if err != nil {
				return err
			}
			defer sess.Close()
			return runRegPoll(cmd, g, sess, &regPollSpec{
				host: host, path: path, unit: unit, fn: fn, kind: kind,
				addr: addr, count: count, interval: interval, times: times,
			})
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", time.Second, "轮询间隔")
	cmd.Flags().IntVar(&times, "times", 0, "轮询次数(0=无限,Ctrl-C 退出)")
	cmd.Flags().StringVar(&opt.kind, "kind", "holding", "读取类型:holding|input")
	return cmd
}

func runRegPoll(cmd *cobra.Command, g *globalFlags, sess *regSession, spec *regPollSpec) error {
	ctx := cmd.Context()
	if !g.quiet && !g.jsonOut {
		cmd.PrintErrf("轮询 %s(mode=%s unit=%d %s@0x%04X×%d)每 %s,Ctrl-C 退出...\n",
			spec.path.Address, spec.path.Mode, spec.unit, spec.kind, spec.addr, spec.count, spec.interval)
	}
	ticker := time.NewTicker(spec.interval)
	defer ticker.Stop()
	for i := 0; spec.times == 0 || i < spec.times; i++ {
		values, reconnected, err := sess.read(ctx, spec.unit, spec.fn, spec.addr, spec.count)
		if ctx.Err() != nil {
			return nil
		}
		if reconnected && !g.quiet && !g.jsonOut {
			cmd.PrintErrln(yellow("(连接已重建)"))
		}
		if err != nil {
			// 单次失败报告但不中断:下个周期设备可能恢复。ctx 取消才退出(上方已判)。
			cmd.PrintErrln("错误: " + err.Error())
		} else {
			emitRegPollSample(cmd, g, spec, values)
		}
		if spec.times != 0 && i == spec.times-1 {
			break
		}
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
		}
	}
	return nil
}

// emitRegPollSample 输出一次采样:JSON 模式逐次一个对象,否则紧凑单行便于滚动监控。
func emitRegPollSample(cmd *cobra.Command, g *globalFlags, spec *regPollSpec, values []uint16) {
	out := cmd.OutOrStdout()
	if g.jsonOut {
		_ = writeJSONLine(out, regResultJSON{
			SchemaVersion: jsonSchemaVersion,
			Target:        spec.host,
			DataAddress:   spec.path.Address,
			Mode:          string(spec.path.Mode),
			WorkMode:      spec.path.WorkMode,
			AppProto:      spec.path.AppProto,
			Unit:          spec.unit,
			Kind:          spec.kind,
			Address:       spec.addr,
			Count:         len(values),
			Values:        regRows(spec.addr, values),
		})
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "[%s] unit=%d %s", time.Now().Format("15:04:05"), spec.unit, spec.kind)
	for i, v := range values {
		fmt.Fprintf(&b, " 0x%04X=%d", spec.addr+uint16(i), v)
	}
	fmt.Fprintln(out, b.String())
}

func writeJSONLine(w interface {
	Write([]byte) (int, error)
}, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	b = append(b, '\n')
	_, err = w.Write(b)
	return err
}
