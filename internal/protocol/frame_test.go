package protocol

import (
	"bytes"
	"testing"
)

// TestSerialEncodeGolden 用串口文档第 3 节的精确命令样例钉死帧格式与字段偏移。
// 期望值是不含 10 字节 magic 的尾部。
func TestSerialEncodeGolden(t *testing.T) {
	cases := []struct {
		name     string
		build    func() ([]byte, error)
		wantTail string
	}{
		{"读 dev_name §3.9", func() ([]byte, error) { return EncodeSerialRead(38, 10) }, "00 26 0a"},
		{"读 devid §3.11", func() ([]byte, error) { return EncodeSerialRead(31, 6) }, "00 1f 06"},
		{"读 ver §3.14", func() ([]byte, error) { return EncodeSerialRead(103, 1) }, "00 67 01"},
		{"读 status §3.1", func() ([]byte, error) { return EncodeSerialRead(61, 1) }, "00 3d 01"},
		{"写 dns §3.4", func() ([]byte, error) { return EncodeSerialWrite(SerialCmdWrite, 62, []byte{8, 8, 8, 8}) }, "01 3e 04 08 08 08 08"},
		{"写 dest_port §3.13", func() ([]byte, error) { return EncodeSerialWrite(SerialCmdWrite, 18, []byte{0x04, 0x01}) }, "01 12 02 04 01"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.build()
			if err != nil {
				t.Fatal(err)
			}
			want := append(append([]byte(nil), serialMagic[:]...), mustHex(t, tc.wantTail)...)
			if !bytes.Equal(got, want) {
				t.Fatalf("\n got  %x\n want %x", got, want)
			}
		})
	}
}

func TestSerialBounds(t *testing.T) {
	if _, err := EncodeSerialRead(160, 10); err == nil { // 160+10 > 167
		t.Fatal("越界应报错")
	}
	if _, err := EncodeSerialWrite(SerialCmdWrite, 0, make([]byte, ParamLen+1)); err == nil {
		t.Fatal("超长应报错")
	}
}

func TestUDPFrameRoundTrip(t *testing.T) {
	var p Param
	if err := p.SetField("local_ip", "192.168.1.169"); err != nil {
		t.Fatal(err)
	}
	frame := EncodeUDP(UDPCmdModifyParam, p)
	if len(frame) != 3+ParamLen {
		t.Fatalf("帧长 %d", len(frame))
	}
	if frame[0] != 0x5a || frame[1] != 0x4c || frame[2] != 0x02 {
		t.Fatalf("帧头错误 %x", frame[:3])
	}
	cmd, p2, err := DecodeUDP(frame)
	if err != nil {
		t.Fatal(err)
	}
	if cmd != UDPCmdModifyParam {
		t.Fatalf("cmd %x", cmd)
	}
	if p2 != p {
		t.Fatal("参数往返不一致")
	}
}

func TestEncodeBroadcastQuery(t *testing.T) {
	b := EncodeBroadcastQuery()
	if len(b) != 3+ParamLen {
		t.Fatalf("帧长 %d", len(b))
	}
	if b[0] != 0x5a || b[1] != 0x4c || b[2] != 0x00 {
		t.Fatalf("帧头 %x", b[:3])
	}
	for i := 3; i < len(b); i++ {
		if b[i] != 0 {
			t.Fatal("查询帧参数区应全 0")
		}
	}
}

func TestDecodeUDPBad(t *testing.T) {
	full := make([]byte, 3+ParamLen)
	full[0], full[1] = 0x00, 0x00 // magic 错
	cases := [][]byte{
		nil,
		{0x5a},
		{0x5a, 0x4c, 0x00}, // 过短
		append([]byte{0x5a, 0x4c, 0x01}, make([]byte, 100)...), // magic 对但参数区不足 167(防短包清零回写)
		full, // magic 错
	}
	for i, b := range cases {
		if _, _, err := DecodeUDP(b); err == nil {
			t.Errorf("case %d 应报错", i)
		}
	}
}

func TestChangedSegments(t *testing.T) {
	var p Param
	mustSet(t, &p, "recon_time", "12")
	mustSet(t, &p, "keep_alive", "60")

	// 相邻字段合并成一段
	segs, err := ChangedSegments(&p, []string{"recon_time", "keep_alive"})
	if err != nil {
		t.Fatal(err)
	}
	if len(segs) != 1 || segs[0].Offset != 96 || !bytes.Equal(segs[0].Data, []byte{0x0c, 0x3c}) {
		t.Fatalf("相邻应合并为 1 段, got %+v", segs)
	}

	// 不相邻分两段
	mustSet(t, &p, "local_ip", "192.168.1.1")
	segs2, err := ChangedSegments(&p, []string{"local_ip", "dest_port"})
	if err != nil {
		t.Fatal(err)
	}
	if len(segs2) != 2 {
		t.Fatalf("不相邻应 2 段, got %d", len(segs2))
	}

	// 位域折算为容器字节段
	mustSet(t, &p, "func_en.need_password", "1")
	segs3, err := ChangedSegments(&p, []string{"func_en.need_password"})
	if err != nil {
		t.Fatal(err)
	}
	if len(segs3) != 1 || segs3[0].Offset != 110 || len(segs3[0].Data) != 1 {
		t.Fatalf("位域段错误 %+v", segs3)
	}

	if _, err := ChangedSegments(&p, []string{"nope"}); err == nil {
		t.Fatal("未知字段应报错")
	}
}

func TestFullSegment(t *testing.T) {
	var p Param
	segs := FullSegment(&p)
	if len(segs) != 1 || segs[0].Offset != 0 || len(segs[0].Data) != ParamLen {
		t.Fatalf("整块段错误 %+v", segs)
	}
	// 独立副本:改 p 不应影响已生成段
	p[0] = 0xff
	if segs[0].Data[0] == 0xff {
		t.Fatal("段数据应是独立副本")
	}
}

func mustSet(t *testing.T, p *Param, field, value string) {
	t.Helper()
	if err := p.SetField(field, value); err != nil {
		t.Fatalf("SetField %s=%s: %v", field, value, err)
	}
}
