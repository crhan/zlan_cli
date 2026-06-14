package transport

import (
	"errors"
	"net"
	"testing"
	"time"

	"zlan/internal/protocol"
)

func TestDirectedBroadcast(t *testing.T) {
	cases := []struct{ ip, mask, want string }{
		{"192.168.1.10", "255.255.255.0", "192.168.1.255"},
		{"10.0.0.5", "255.0.0.0", "10.255.255.255"},
		{"172.16.5.4", "255.255.0.0", "172.16.255.255"},
		{"192.168.1.10", "255.255.255.255", "192.168.1.10"},
	}
	for _, tc := range cases {
		ip := net.ParseIP(tc.ip)
		mask := net.IPMask(net.ParseIP(tc.mask).To4())
		got := directedBroadcast(ip, mask)
		if got.String() != tc.want {
			t.Errorf("%s/%s: got %v want %s", tc.ip, tc.mask, got, tc.want)
		}
	}
}

// fakeDevice 起一个回环 UDP 设备:收到查询(0x00/0x04)回应答(0x01)。
func fakeDevice(t *testing.T, reply protocol.Param) (*net.UDPAddr, func()) {
	t.Helper()
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatalf("起 fake 设备失败: %v", err)
	}
	done := make(chan struct{})
	go func() {
		buf := make([]byte, udpReadBuf)
		for {
			_ = conn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
			n, src, err := conn.ReadFromUDP(buf)
			if err != nil {
				select {
				case <-done:
					return
				default:
					continue
				}
			}
			cmd, _, derr := protocol.DecodeUDP(buf[:n])
			if derr != nil {
				continue
			}
			if cmd == protocol.UDPCmdBroadcastQuery || cmd == protocol.UDPCmdUnicastQuery {
				_, _ = conn.WriteToUDP(protocol.EncodeUDP(protocol.UDPCmdDeviceReply, reply), src)
			}
		}
	}()
	return conn.LocalAddr().(*net.UDPAddr), func() { close(done); conn.Close() }
}

func TestUDPConnReadParam(t *testing.T) {
	var want protocol.Param
	mustSet(t, &want, "local_ip", "192.168.1.50")
	mustSet(t, &want, "dev_name", "TESTDEV")

	addr, stop := fakeDevice(t, want)
	defer stop()

	c, err := Dial(addr, 500*time.Millisecond, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	got, err := c.ReadParam()
	if err != nil {
		t.Fatalf("ReadParam: %v", err)
	}
	if got != want {
		t.Fatal("读回参数与设备应答不一致")
	}
}

func TestUDPConnNoDevice(t *testing.T) {
	// 黑洞:只收不回,避免 connected socket 收到 ICMP unreachable 而非超时。
	black, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer black.Close()

	c, err := Dial(black.LocalAddr().(*net.UDPAddr), 80*time.Millisecond, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	if _, err := c.ReadParam(); !errors.Is(err, ErrNoDevice) {
		t.Fatalf("期望 ErrNoDevice,得到 %v", err)
	}
}

func TestUDPConnWriteParam(t *testing.T) {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	gotCmd := make(chan protocol.UDPCmd, 1)
	go func() {
		buf := make([]byte, udpReadBuf)
		_ = conn.SetReadDeadline(time.Now().Add(time.Second))
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			return
		}
		cmd, _, _ := protocol.DecodeUDP(buf[:n])
		gotCmd <- cmd
	}()

	c, err := Dial(conn.LocalAddr().(*net.UDPAddr), 500*time.Millisecond, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	var p protocol.Param
	if err := c.WriteParam(p, nil, WritePersist); err != nil {
		t.Fatalf("WriteParam: %v", err)
	}
	select {
	case cmd := <-gotCmd:
		if cmd != protocol.UDPCmdModifyParam {
			t.Fatalf("期望 0x02,得到 0x%02x", cmd)
		}
	case <-time.After(time.Second):
		t.Fatal("设备未收到写命令")
	}
}

func TestMonitor(t *testing.T) {
	port := freePort(t)
	ctx := t.Context()

	got := make(chan string, 1)
	go func() {
		_ = Monitor(ctx, port, func(src *net.UDPAddr, cmd protocol.UDPCmd, p protocol.Param) []byte {
			name, _ := p.GetField("dev_name")
			got <- name
			return nil
		})
	}()
	time.Sleep(50 * time.Millisecond) // 等 Monitor 绑好端口

	var p protocol.Param
	mustSet(t, &p, "dev_name", "MON1")
	sender, err := net.DialUDP("udp4", nil, &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: port})
	if err != nil {
		t.Fatal(err)
	}
	defer sender.Close()
	if _, err := sender.Write(protocol.EncodeUDP(protocol.UDPCmdDeviceReply, p)); err != nil {
		t.Fatal(err)
	}

	select {
	case name := <-got:
		if name != "MON1" {
			t.Fatalf("handler 收到 dev_name=%q", name)
		}
	case <-time.After(time.Second):
		t.Fatal("Monitor 未在超时内处理上报包")
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	c, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		t.Fatal(err)
	}
	port := c.LocalAddr().(*net.UDPAddr).Port
	c.Close()
	return port
}

func mustSet(t *testing.T, p *protocol.Param, field, value string) {
	t.Helper()
	if err := p.SetField(field, value); err != nil {
		t.Fatalf("SetField %s=%s: %v", field, value, err)
	}
}
