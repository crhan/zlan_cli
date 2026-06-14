package transport

import (
	"testing"
	"time"

	"zlan/internal/protocol"
)

// fakeSerial 模拟一台串口设备:读命令(cmd bit0=0)按 pos/len 返回参数区,
// 写命令(bit0=1)把 data 写入自己的参数;writeFrame 前的 ResetInputBuffer 清待读区。
type fakeSerial struct {
	param   protocol.Param
	writes  [][]byte
	pending []byte
}

func (f *fakeSerial) Write(b []byte) (int, error) {
	f.writes = append(f.writes, append([]byte(nil), b...))
	if len(b) < 13 {
		return len(b), nil
	}
	cmd, pos, length := b[10], int(b[11]), int(b[12])
	if cmd&1 == 0 { // 读
		f.pending = append(f.pending, f.param[pos:pos+length]...)
	} else { // 写
		copy(f.param[pos:pos+(len(b)-13)], b[13:])
	}
	return len(b), nil
}

func (f *fakeSerial) Read(p []byte) (int, error) {
	if len(f.pending) == 0 {
		return 0, nil // 模拟无数据(串口读超时返回 0,nil)
	}
	n := copy(p, f.pending)
	f.pending = f.pending[n:]
	return n, nil
}

func (f *fakeSerial) ResetInputBuffer() error            { f.pending = nil; return nil }
func (f *fakeSerial) SetReadTimeout(time.Duration) error { return nil }
func (f *fakeSerial) Close() error                       { return nil }

func newFakeConn(f *fakeSerial) *SerialConn {
	return &SerialConn{port: f, chunk: 64, timeout: time.Second} // segDelay=0,测试不 sleep
}

func TestSerialReadParam(t *testing.T) {
	var dev protocol.Param
	mustSet(t, &dev, "local_ip", "10.1.2.3")
	mustSet(t, &dev, "dev_name", "SER1")
	mustSet(t, &dev, "recon_time", "12")
	mustSet(t, &dev, "keep_alive", "60")

	f := &fakeSerial{param: dev}
	got, err := newFakeConn(f).ReadParam()
	if err != nil {
		t.Fatalf("ReadParam: %v", err)
	}
	if got != dev {
		t.Fatal("分段读拼接结果与设备参数不一致")
	}
	if len(f.writes) != 3 { // 167 / 64 = 3 段
		t.Fatalf("期望 3 段读命令,实际 %d", len(f.writes))
	}
}

func TestSerialWriteParamPersist(t *testing.T) {
	f := &fakeSerial{}
	var p protocol.Param
	mustSet(t, &p, "baud", "115200")
	if err := newFakeConn(f).WriteParam(p, []string{"baud"}, WritePersist); err != nil {
		t.Fatalf("WriteParam: %v", err)
	}
	if f.param[37] != 0x0b {
		t.Fatalf("baud 未写入设备,got %02x", f.param[37])
	}
	if len(f.writes) != 1 {
		t.Fatalf("单字段应 1 帧,got %d", len(f.writes))
	}
	if f.writes[0][10] != byte(protocol.SerialCmdWriteSave) {
		t.Fatalf("持久写应为 0x03,got 0x%02x", f.writes[0][10])
	}
}

func TestSerialWriteParamVolatile(t *testing.T) {
	f := &fakeSerial{}
	var p protocol.Param
	mustSet(t, &p, "baud", "9600")
	if err := newFakeConn(f).WriteParam(p, []string{"baud"}, WriteVolatile); err != nil {
		t.Fatalf("WriteParam: %v", err)
	}
	if f.writes[0][10] != byte(protocol.SerialCmdWrite) {
		t.Fatalf("临时写应为 0x01,got 0x%02x", f.writes[0][10])
	}
}

func TestSerialReboot(t *testing.T) {
	f := &fakeSerial{}
	if err := newFakeConn(f).Reboot(); err != nil {
		t.Fatalf("Reboot: %v", err)
	}
	w := f.writes[0]
	if w[10] != 0x07 || w[11] != 0x1f || w[12] != 0x01 || w[13] != 0x00 {
		t.Fatalf("reboot 帧应为 07 1f 01 00,got %02x %02x %02x %02x", w[10], w[11], w[12], w[13])
	}
}
