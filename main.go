// Command zlan 管理 ZLAN(卓岚)串口服务器/联网模块。
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"zlan/internal/cli"
)

func main() {
	// Ctrl-C / SIGTERM 取消 ctx,长操作据此快速退出。
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// 唯一的 os.Exit:子命令一律 RunE 返回 error,这里统一打印并映射退出码。
	if err := cli.NewRootCmd().ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "错误:", err)
		os.Exit(cli.ExitCode(err))
	}
}
