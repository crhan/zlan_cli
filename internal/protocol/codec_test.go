package protocol

import (
	"bytes"
	"encoding/hex"
	"strings"
	"testing"
)

// mustHex 把形如 "01 12 02" 的字符串解析为字节。
func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(strings.ReplaceAll(s, " ", ""))
	if err != nil {
		t.Fatalf("非法 hex %q: %v", s, err)
	}
	return b
}

func TestUnmarshalLen(t *testing.T) {
	if _, err := Unmarshal(make([]byte, ParamLen)); err != nil {
		t.Fatalf("正确长度不应报错: %v", err)
	}
	if _, err := Unmarshal(make([]byte, 10)); err == nil {
		t.Fatal("错误长度应报错")
	}
}

// TestFieldSetGolden 把字段写入后的字节与文档 golden 比对,并验证 get 往返。
// 偏移与编码源自 SPEC §3/§16;recon/keep_alive 两条同时固化 §9 裁决。
func TestFieldSetGolden(t *testing.T) {
	cases := []struct {
		field, value string
		off          int
		want         string // 期望字节(hex)
	}{
		{"local_ip", "192.168.1.200", 0, "c0 a8 01 c8"},
		{"net_mask", "255.255.255.0", 4, "ff ff ff 00"},
		{"gateway", "192.168.1.1", 8, "c0 a8 01 01"},
		{"local_port", "4196", 16, "10 64"},
		{"dest_port", "80", 18, "00 50"},
		{"work_mode", "tcp-client", 20, "01"},
		{"baud", "115200", 37, "0b"},
		{"baud", "7200", 37, "03"}, // 非标准档,查表
		{"baud", "9600", 37, "04"},
		{"parity", "none", 48, "00"},
		{"packing_len", "1300", 50, "05 14"},
		{"dhcp_en", "dhcp", 56, "01"},
		{"data_bits", "8", 59, "00"}, // 反序:0=8bit
		{"data_bits", "5", 59, "03"},
		{"app_proto", "modbus", 60, "01"},
		{"dns_server_ip", "8.8.8.8", 62, "08 08 08 08"},
		{"recon_time", "12", 96, "0c"}, // §9:重连在前
		{"keep_alive", "60", 97, "3c"}, // §9:保活在后
		{"web_port", "80", 98, "00 50"},
		{"group_ip", "230.90.76.1", 105, "e6 5a 4c 01"},
	}
	for _, tc := range cases {
		t.Run(tc.field+"="+tc.value, func(t *testing.T) {
			var p Param
			if err := p.SetField(tc.field, tc.value); err != nil {
				t.Fatalf("SetField: %v", err)
			}
			want := mustHex(t, tc.want)
			if got := p[tc.off : tc.off+len(want)]; !bytes.Equal(got, want) {
				t.Fatalf("offset %d: got %x want %x", tc.off, got, want)
			}
			back, err := p.GetField(tc.field)
			if err != nil {
				t.Fatalf("GetField: %v", err)
			}
			if back != tc.value {
				t.Errorf("往返不一致: set %q get %q", tc.value, back)
			}
		})
	}
}

func TestCStringField(t *testing.T) {
	var p Param
	if err := p.SetField("dev_name", "ZLDEV1"); err != nil {
		t.Fatal(err)
	}
	if got, _ := p.GetField("dev_name"); got != "ZLDEV1" {
		t.Fatalf("got %q", got)
	}
	if p[38+6] != 0 {
		t.Fatal("字符串后应有结尾 0")
	}
	// 设短名要清掉旧残留
	if err := p.SetField("dev_name", "ab"); err != nil {
		t.Fatal(err)
	}
	if got, _ := p.GetField("dev_name"); got != "ab" {
		t.Fatalf("清尾后 got %q", got)
	}
	if p[38+2] != 0 {
		t.Fatal("未清掉旧残留字节")
	}
	// dev_name 区 10 字节,10 个可见字符无处放结尾 0,应拒绝
	if err := p.SetField("dev_name", "0123456789"); err == nil {
		t.Fatal("超长应拒绝")
	}
}

func TestVersionFormat(t *testing.T) {
	var p Param
	p[103] = 117
	if got, _ := p.GetField("ver"); got != "1.500" { // 383+117
		t.Fatalf("got %q", got)
	}
	p[103] = 0x8a
	if got, _ := p.GetField("ver"); got != "1.521" { // 383+138
		t.Fatalf("got %q", got)
	}
}

func TestDevIDFormat(t *testing.T) {
	var p Param
	copy(p[31:37], mustHex(t, "5a 4c 6f 73 cc d6"))
	if got, _ := p.GetField("devid"); got != "5a:4c:6f:73:cc:d6" {
		t.Fatalf("got %q", got)
	}
	if p.DevID() != [6]byte{0x5a, 0x4c, 0x6f, 0x73, 0xcc, 0xd6} {
		t.Fatal("DevID 不一致")
	}
}

func TestBitField(t *testing.T) {
	var p Param
	if err := p.SetField("func_en.need_password", "1"); err != nil {
		t.Fatal(err)
	}
	if p[110] != 0x04 {
		t.Fatalf("need_password 后 got %02x", p[110])
	}
	if err := p.SetField("func_en.data_restart", "1"); err != nil {
		t.Fatal(err)
	}
	if p[110] != 0x05 {
		t.Fatalf("叠加后 got %02x", p[110])
	}
	if got, _ := p.GetField("func_en"); got != "0x05" {
		t.Fatalf("容器 got %q", got)
	}
	if got, _ := p.GetField("func_en.need_password"); got != "1" {
		t.Fatalf("子位 got %q", got)
	}
	if err := p.SetField("func_en.need_password", "0"); err != nil {
		t.Fatal(err)
	}
	if p[110] != 0x01 {
		t.Fatalf("清位后 got %02x", p[110])
	}
}

func TestReadOnlyAndOpaqueRejected(t *testing.T) {
	var p Param
	for _, name := range []string{"ver", "devid", "status", "func_sel", "func_sel2", "status.connected", "func_sel.dns"} {
		if err := p.SetField(name, "1"); err == nil {
			t.Errorf("只读字段 %s 应拒绝写入", name)
		}
	}
	for _, name := range []string{"reserve", "user_param"} {
		if err := p.SetField(name, "00"); err == nil {
			t.Errorf("保留区 %s 应拒绝直接设置", name)
		}
	}
}

func TestSetFieldValidation(t *testing.T) {
	var p Param
	bad := []struct{ field, value string }{
		{"local_ip", "999.1.1.1"},
		{"work_mode", "bogus"},
		{"baud", "12345"},
		{"packing_len", "0"},        // 下界
		{"packing_len", "2000"},     // 上界
		{"group_ip", "192.168.1.1"}, // 非组播段
		{"dest_port", "70000"},
		{"unknown_field", "x"},
	}
	for _, tc := range bad {
		if err := p.SetField(tc.field, tc.value); err == nil {
			t.Errorf("%s=%q 应被拒绝", tc.field, tc.value)
		}
	}
}
