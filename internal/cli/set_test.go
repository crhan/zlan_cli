package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"zlan/internal/device"
)

// TestRenderSetResultNetworkBeatsReboot 锁死 switch 顺序:UDP 改网络字段时
// NetworkField 与 RebootExpected 同真,必须显示更具体的网段警告而非泛化重启提示。
func TestRenderSetResultNetworkBeatsReboot(t *testing.T) {
	var out, errBuf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)
	cmd.SetErr(&errBuf)
	res := &device.SetResult{
		Changed:        []string{"local_ip"},
		NetworkField:   true,
		RebootExpected: true,
	}
	if err := renderSetResult(cmd, &globalFlags{}, res); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errBuf.String(), "网段") {
		t.Fatalf("应显示网段警告, got: %q", errBuf.String())
	}
	if strings.Contains(errBuf.String(), "稍后用 info 确认") {
		t.Fatalf("不应被泛化重启提示吞掉, got: %q", errBuf.String())
	}
}

func TestProfileAssignmentsModbusTCPRTU(t *testing.T) {
	got, err := profileAssignments("modbus-tcp-rtu")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		"work_mode":  "tcp-server",
		"local_port": "502",
		"app_proto":  "modbus",
		"baud":       "9600",
		"parity":     "none",
		"data_bits":  "8",
	}
	for k, v := range want {
		if got[k] != v {
			t.Fatalf("%s=%q, want %q", k, got[k], v)
		}
	}
}

func TestMergeAssignmentsOverride(t *testing.T) {
	got := mergeAssignments(
		map[string]string{"baud": "9600", "parity": "none"},
		map[string]string{"baud": "115200"},
	)
	if got["baud"] != "115200" {
		t.Fatalf("override baud=%q", got["baud"])
	}
	if got["parity"] != "none" {
		t.Fatalf("base parity=%q", got["parity"])
	}
}

func TestParseWriteArgsAllowEmptyForProfile(t *testing.T) {
	g := &globalFlags{}
	host, kvs, err := parseWriteArgs(g, []string{"192.168.1.200"}, true)
	if err != nil {
		t.Fatal(err)
	}
	if host != "192.168.1.200" {
		t.Fatalf("host=%q", host)
	}
	if len(kvs) != 0 {
		t.Fatalf("kvs=%v", kvs)
	}
}
