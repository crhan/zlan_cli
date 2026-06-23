package cli

import (
	"bytes"
	"encoding/json"
	"net"
	"strings"
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

func TestRenderParamDisplaysDecodedUserParamWithoutRawDuplicates(t *testing.T) {
	var p protocol.Param
	records := []protocol.UserTLV{
		{Type: protocol.UserTLVSerialTxBytes, Value: []byte{0x00, 0x00, 0x00, 0x10}},
		{Type: protocol.UserTLVSerialRxBytes, Value: []byte{0x00, 0x01, 0x40, 0x97}},
		{Type: protocol.UserTLVWiFiSSID, Value: []byte("Roland")},
		{Type: protocol.UserTLVWiFiChannel, Value: []byte{0x44}},
		{Type: protocol.UserTLVWiFiModeCrypt, Value: []byte{0x86}},
		{Type: protocol.UserTLVWiFiPassword, Value: []byte("secret")},
	}
	if err := p.SetUserTLVs(records); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := renderParam(&buf, &p, false); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, want := range []string{"[wifi]", "ssid", "Roland", "mode", "sta", "crypt", "auto", "key", "<set>"} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderParam missing %q:\n%s", want, got)
		}
	}
	for _, want := range []string{"serial_tx_bytes", "16", "serial_rx_bytes", "82071"} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderParam missing %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"[user_param_tlv]", "name=wifi_ssid", "name=serial_tx_bytes", "wifi_password"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("renderParam has duplicated raw TLV %q:\n%s", unwanted, got)
		}
	}
	if strings.Contains(got, "secret") {
		t.Fatalf("renderParam leaked WiFi key:\n%s", got)
	}

	var jsonBuf bytes.Buffer
	if err := renderParam(&jsonBuf, &p, true); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(jsonBuf.Bytes(), &doc); err != nil {
		t.Fatal(err)
	}
	if _, ok := doc["user_param_tlvs"]; ok {
		t.Fatalf("json should not include raw TLVs when all user_param records are decoded:\n%s", jsonBuf.String())
	}
	if _, ok := doc["wifi"]; !ok {
		t.Fatalf("json missing decoded wifi:\n%s", jsonBuf.String())
	}
	if _, ok := doc["serial_counters"]; !ok {
		t.Fatalf("json missing decoded serial counters:\n%s", jsonBuf.String())
	}
}

func TestRenderParamIncludesOnlyUnhandledUserParamTLVs(t *testing.T) {
	var p protocol.Param
	records := []protocol.UserTLV{
		{Type: protocol.UserTLVWiFiSSID, Value: []byte("Roland")},
		{Type: protocol.UserTLVWiFiChannel, Value: []byte{0x44}},
		{Type: protocol.UserTLVWiFiModeCrypt, Value: []byte{0x86}},
		{Type: protocol.UserTLVWiFiPassword, Value: []byte("secret")},
		{Type: protocol.UserTLVCustom, Value: []byte{0x01, 0x02}},
	}
	if err := p.SetUserTLVs(records); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := renderParam(&buf, &p, false); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	for _, want := range []string{"[wifi]", "[user_param_tlv]", "type=255", "name=custom", "value=0102"} {
		if !strings.Contains(got, want) {
			t.Fatalf("renderParam missing %q:\n%s", want, got)
		}
	}
	for _, unwanted := range []string{"name=wifi_ssid", "name=wifi_channel", "name=wifi_mode_crypt", "name=wifi_password", "secret"} {
		if strings.Contains(got, unwanted) {
			t.Fatalf("renderParam included handled TLV %q:\n%s", unwanted, got)
		}
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
