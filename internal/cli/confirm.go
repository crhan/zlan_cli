package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// confirm 在 TTY 下提示确认(--yes 直接通过);非 TTY 且无 --yes 则报错而非阻塞。
// 返回 proceed 表示用户是否同意继续。
func confirm(cmd *cobra.Command, g *globalFlags, format string, a ...any) (proceed bool, err error) {
	if g.yes {
		return true, nil
	}
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return false, exitErr(ExitUsage, fmt.Errorf("非交互环境拒绝提示确认;确认请加 --yes"))
	}
	cmd.PrintErrf(format+" [y/N]: ", a...)
	line, _ := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	ans := strings.ToLower(strings.TrimSpace(line))
	return ans == "y" || ans == "yes", nil
}
