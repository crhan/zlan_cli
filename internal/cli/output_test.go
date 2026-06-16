package cli

import (
	"bytes"
	"net"
	"testing"

	"zlan/internal/protocol"
	"zlan/internal/transport"
)

func TestRenderDevicesAlignsCJKNames(t *testing.T) {
	oldColor := useColor
	useColor = false
	defer func() { useColor = oldColor }()

	devs := []transport.Device{
		testDevice(t, "04:ee:e8:1a:07:29", "楼顶水表", "192.168.15.44", "tcp-server", "9600", 396, false),
		testDevice(t, "28:52:75:f3:8e:9e", "1401新风", "192.168.15.42", "tcp-server", "9600", 528, false),
		testDevice(t, "04:ee:e8:1a:06:09", "热水循环", "192.168.15.40", "tcp-server", "9600", 396, true),
		testDevice(t, "28:52:20:39:ae:2f", "1803新风", "192.168.15.43", "tcp-server", "9600", 473, true),
	}

	var buf bytes.Buffer
	if err := renderDevices(&buf, devs, false); err != nil {
		t.Fatal(err)
	}
	want := "" +
		"DEVID              NAME      IP             MODE        BAUD  VER    STATUS\n" +
		"04:ee:e8:1a:07:29  楼顶水表  192.168.15.44  tcp-server  9600  1.396  idle\n" +
		"28:52:75:f3:8e:9e  1401新风  192.168.15.42  tcp-server  9600  1.528  idle\n" +
		"04:ee:e8:1a:06:09  热水循环  192.168.15.40  tcp-server  9600  1.396  connected\n" +
		"28:52:20:39:ae:2f  1803新风  192.168.15.43  tcp-server  9600  1.473  connected\n"
	if got := buf.String(); got != want {
		t.Fatalf("renderDevices() mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestDisplayWidthCountsCJKAndSkipsANSI(t *testing.T) {
	if got := displayWidth("1401新风"); got != 8 {
		t.Fatalf("mixed CJK width=%d, want 8", got)
	}
	if got := displayWidth("\x1b[32mconnected\x1b[0m"); got != len("connected") {
		t.Fatalf("ANSI width=%d, want %d", got, len("connected"))
	}
}

func testDevice(t *testing.T, mac, name, ip, mode, baud string, version int, connected bool) transport.Device {
	t.Helper()
	var p protocol.Param
	hw, err := net.ParseMAC(mac)
	if err != nil {
		t.Fatal(err)
	}
	copy(p[31:37], hw)
	for field, value := range map[string]string{
		"dev_name":  name,
		"local_ip":  ip,
		"work_mode": mode,
		"baud":      baud,
	} {
		if err := p.SetField(field, value); err != nil {
			t.Fatalf("SetField %s=%s: %v", field, value, err)
		}
	}
	p[103] = byte(version - 383)
	if connected {
		p[61] = 1
	}
	return transport.Device{
		Addr:  &net.UDPAddr{IP: net.ParseIP(ip), Port: protocol.MgmtPort},
		Param: p,
	}
}
