package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"zlan/internal/device"
	"zlan/internal/modbus"
)

// regSession 在 ZLAN 数据通道上维持一条 Modbus 长连接,断线时按需重连。
//
// Modbus 事务非并发安全(一条连接同一时刻只能跑一个事务),调用方需串行使用——
// REPL 单循环、poll 单 ticker 均满足。保活策略是"重连兜底"而非主动 ping:设备
// keep_alive 空闲超时断连后,下次请求触发重连,对调用方透明。详见 CLAUDE.md。
type regSession struct {
	address string
	mode    modbus.Mode
	timeout time.Duration
	client  *modbus.Client
}

// newRegSession 解析出的数据通道路径建立首条连接。
func newRegSession(ctx context.Context, path regDataPath, timeout time.Duration) (*regSession, error) {
	s := &regSession{address: path.Address, mode: path.Mode, timeout: timeout}
	if err := s.dial(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *regSession) dial(ctx context.Context) error {
	client, err := modbus.DialTCP(ctx, s.address, s.mode, s.timeout)
	if err != nil {
		return err
	}
	s.client = client
	return nil
}

// Close 释放底层连接。
func (s *regSession) Close() error {
	if s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *regSession) reconnect(ctx context.Context) error {
	_ = s.client.Close()
	return s.dial(ctx)
}

// read 读寄存器;遇连接级断开自动重连一次重试。reconnected 报告是否发生过重连。
// Modbus 读是幂等的,重发安全。
func (s *regSession) read(ctx context.Context, unit, fn byte, addr, count uint16) (vals []uint16, reconnected bool, err error) {
	vals, err = s.client.ReadRegisters(unit, fn, addr, count)
	if err == nil || !isConnError(err) {
		return vals, false, err
	}
	if err = s.reconnect(ctx); err != nil {
		return nil, true, err
	}
	vals, err = s.client.ReadRegisters(unit, fn, addr, count)
	return vals, true, err
}

// write 写寄存器;遇连接级断开自动重连一次重试。写寄存器是写定值,幂等,重发安全。
func (s *regSession) write(ctx context.Context, unit byte, addr uint16, values []uint16) (reconnected bool, err error) {
	err = s.client.WriteRegisters(unit, addr, values)
	if err == nil || !isConnError(err) {
		return false, err
	}
	if err = s.reconnect(ctx); err != nil {
		return true, err
	}
	return true, s.client.WriteRegisters(unit, addr, values)
}

// isConnError 判断错误是否为 TCP 连接级断开(对端 FIN/RST 或本地已关闭),据此触发重连。
// 设备 keep_alive 空闲超时后会断连,典型表现:下次读得到 EOF、下次写得到 EPIPE/ECONNRESET。
// I/O 超时不算——那可能是设备忙或 unit 不对,静默重连会掩盖真实问题,直接报给用户。
func isConnError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, io.EOF) ||
		errors.Is(err, io.ErrUnexpectedEOF) ||
		errors.Is(err, syscall.EPIPE) ||
		errors.Is(err, syscall.ECONNRESET) ||
		errors.Is(err, net.ErrClosed)
}

// resolveRegSessionPath 走管理通道读一次参数、解析出数据通道地址/协议,随即关闭管理通道。
// 数据通道长连接独立持有,不让管理通道(UDP/串口)空占。
func resolveRegSessionPath(cmd *cobra.Command, g *globalFlags, host string, opt *regOptions) (regDataPath, error) {
	var path regDataPath
	err := withHost(cmd, g, host, func(ep *device.Endpoint) error {
		p, rerr := readRegParam(cmd, g, ep, host)
		if rerr != nil {
			return rerr
		}
		path, rerr = resolveRegDataPath(&p, opt)
		return rerr
	})
	return path, err
}

func newRegSessionCmd(g *globalFlags, opt *regOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "session <target>",
		Aliases: []string{"repl", "shell"},
		Short:   "建立数据通道长连接,交互式反复读写寄存器",
		Long: `建立一条到 ZLAN 数据通道的 TCP 长连接并进入交互提示符,在同一连接上反复读写
寄存器。连接因设备 keep_alive 空闲超时断开时自动重连。Ctrl-D 或 quit 退出。

交互命令:
  read  <addr> [count]      读 holding 寄存器(0x03)
  iread <addr> [count]      读 input 寄存器(0x04)
  write <addr> <value...>   写 holding 寄存器(0x06/0x10)
  unit  <n>                 切换当前 Modbus 从站地址(1..247)
  help                      显示帮助
  quit                      退出(也可 Ctrl-D)

示例:
  zlan reg session 192.168.1.200 --unit 1`,
		Args: func(_ *cobra.Command, args []string) error {
			if g.serial != "" {
				return fmt.Errorf("reg 通过 ZLAN 网络数据通道读写寄存器,不能与 --serial 同用")
			}
			if len(args) != 1 {
				return fmt.Errorf("需要一个设备目标(IP 或 DevID/MAC)")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			unit, err := parseUnit(opt.unit)
			if err != nil {
				return exitErr(ExitUsage, err)
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
			return (&regREPL{cmd: cmd, g: g, sess: sess, host: host, path: path, unit: unit}).run()
		},
	}
	return cmd
}

// regREPL 持有一次交互会话的可变状态(当前 unit 可被 unit 命令改写)。
type regREPL struct {
	cmd  *cobra.Command
	g    *globalFlags
	sess *regSession
	host string
	path regDataPath
	unit byte
}

func (r *regREPL) run() error {
	ctx := r.cmd.Context()
	errw := r.cmd.ErrOrStderr()
	interactive := !r.g.quiet && !r.g.jsonOut
	if interactive {
		fmt.Fprintf(errw, "已连接 %s(mode=%s unit=%d)。输入 help 查看命令,Ctrl-D / quit 退出。\n",
			r.path.Address, r.path.Mode, r.unit)
	}

	// main.go 用 signal.NotifyContext 接管 Ctrl-C,阻塞的 Scan() 不会被 ctx 唤醒;
	// 把读 stdin 放到 goroutine,主循环 select 监 ctx.Done()。
	lines := make(chan string)
	var scanErr error
	go func() {
		sc := bufio.NewScanner(r.cmd.InOrStdin())
		sc.Buffer(make([]byte, 0, 64*1024), 1<<20)
		for sc.Scan() {
			lines <- sc.Text()
		}
		scanErr = sc.Err() // close(lines) 与下方接收构成 happens-before,无 race
		close(lines)
	}()

	for {
		if interactive {
			fmt.Fprintf(errw, "zlan reg[%d]> ", r.unit)
		}
		select {
		case <-ctx.Done():
			if interactive {
				fmt.Fprintln(errw)
			}
			return nil
		case line, ok := <-lines:
			if !ok {
				if interactive {
					fmt.Fprintln(errw)
				}
				return scanErr
			}
			if r.dispatch(line) {
				return nil
			}
		}
	}
}

// dispatch 执行一行交互命令,返回是否应退出。
func (r *regREPL) dispatch(line string) (quit bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return false
	}
	switch strings.ToLower(fields[0]) {
	case "quit", "exit", "q":
		return true
	case "help", "h", "?":
		fmt.Fprint(r.cmd.ErrOrStderr(), regREPLHelp)
	case "unit", "u":
		r.setUnit(fields[1:])
	case "read", "r":
		r.read(modbus.FuncReadHoldingRegisters, "holding", fields[1:])
	case "iread", "ir":
		r.read(modbus.FuncReadInputRegisters, "input", fields[1:])
	case "write", "w":
		r.write(fields[1:])
	default:
		fmt.Fprintln(r.cmd.ErrOrStderr(), yellow("未知命令: "+fields[0]+"(输入 help)"))
	}
	return false
}

func (r *regREPL) setUnit(args []string) {
	errw := r.cmd.ErrOrStderr()
	if len(args) != 1 {
		fmt.Fprintln(errw, yellow("用法: unit <1..247>"))
		return
	}
	v, err := strconv.ParseUint(args[0], 0, 16)
	if err != nil {
		fmt.Fprintln(errw, yellow("非法 unit: "+args[0]))
		return
	}
	u, err := parseUnit(uint(v))
	if err != nil {
		fmt.Fprintln(errw, yellow(err.Error()))
		return
	}
	r.unit = u
	if !r.g.quiet && !r.g.jsonOut {
		fmt.Fprintf(errw, "当前 unit=%d\n", r.unit)
	}
}

func (r *regREPL) read(fn byte, kind string, args []string) {
	errw := r.cmd.ErrOrStderr()
	if len(args) < 1 || len(args) > 2 {
		fmt.Fprintln(errw, yellow("用法: "+readVerb(kind)+" <addr> [count]"))
		return
	}
	addr, err := parseUint16(args[0], "addr")
	if err != nil {
		fmt.Fprintln(errw, yellow(err.Error()))
		return
	}
	count := uint16(1)
	if len(args) == 2 {
		count, err = parseUint16(args[1], "count")
		if err != nil {
			fmt.Fprintln(errw, yellow(err.Error()))
			return
		}
	}
	if err := validateRegRange(addr, int(count), 125); err != nil {
		fmt.Fprintln(errw, yellow(err.Error()))
		return
	}
	values, reconnected, err := r.sess.read(r.cmd.Context(), r.unit, fn, addr, count)
	r.noteReconnect(reconnected)
	if err != nil {
		fmt.Fprintln(errw, "错误: "+err.Error())
		return
	}
	_ = renderRegRead(r.cmd, r.g, r.host, r.path, r.unit, kind, addr, values)
}

func (r *regREPL) write(args []string) {
	errw := r.cmd.ErrOrStderr()
	if len(args) < 2 {
		fmt.Fprintln(errw, yellow("用法: write <addr> <value...>"))
		return
	}
	addr, err := parseUint16(args[0], "addr")
	if err != nil {
		fmt.Fprintln(errw, yellow(err.Error()))
		return
	}
	values, err := parseRegValues(args[1:])
	if err != nil {
		fmt.Fprintln(errw, yellow(err.Error()))
		return
	}
	if err := validateRegRange(addr, len(values), 123); err != nil {
		fmt.Fprintln(errw, yellow(err.Error()))
		return
	}
	reconnected, err := r.sess.write(r.cmd.Context(), r.unit, addr, values)
	r.noteReconnect(reconnected)
	if err != nil {
		fmt.Fprintln(errw, "错误: "+err.Error())
		return
	}
	_ = renderRegWrite(r.cmd, r.g, r.host, r.path, r.unit, addr, values)
}

func (r *regREPL) noteReconnect(reconnected bool) {
	if reconnected && !r.g.quiet && !r.g.jsonOut {
		fmt.Fprintln(r.cmd.ErrOrStderr(), yellow("(连接已重建)"))
	}
}

// readVerb 把 holding/input 映射回对应交互动词,用于用法提示。
func readVerb(kind string) string {
	if kind == "input" {
		return "iread"
	}
	return "read"
}

const regREPLHelp = `交互命令:
  read  <addr> [count]      读 holding 寄存器(0x03)
  iread <addr> [count]      读 input 寄存器(0x04)
  write <addr> <value...>   写 holding 寄存器(0x06/0x10)
  unit  <n>                 切换当前 Modbus 从站地址(1..247)
  help                      显示此帮助
  quit                      退出(也可 Ctrl-D)
地址与值支持 0x 前缀;一次读至多 125、写至多 123 个寄存器。
`
