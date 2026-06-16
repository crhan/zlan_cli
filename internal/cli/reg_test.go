package cli

import (
	"testing"

	"zlan/internal/modbus"
	"zlan/internal/protocol"
)

func TestParseUint16Hex(t *testing.T) {
	got, err := parseUint16("0x0010", "addr")
	if err != nil {
		t.Fatal(err)
	}
	if got != 16 {
		t.Fatalf("got %d", got)
	}
}

func TestParseRegValuesCommaAndSpace(t *testing.T) {
	got, err := parseRegValues([]string{"0x0001,2", "3"})
	if err != nil {
		t.Fatal(err)
	}
	want := []uint16{1, 2, 3}
	if len(got) != len(want) {
		t.Fatalf("len=%d", len(got))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("values=%v", got)
		}
	}
}

func TestResolveRegDataPathAutoModbus(t *testing.T) {
	p := regParam(t, "modbus")
	path, err := resolveRegDataPath(&p, &regOptions{mode: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	if path.Mode != modbus.ModeTCP {
		t.Fatalf("mode=%s", path.Mode)
	}
	if path.Address != "192.168.15.42:502" {
		t.Fatalf("address=%s", path.Address)
	}
}

func TestResolveRegDataPathAutoTransparent(t *testing.T) {
	p := regParam(t, "transparent")
	path, err := resolveRegDataPath(&p, &regOptions{mode: "auto"})
	if err != nil {
		t.Fatal(err)
	}
	if path.Mode != modbus.ModeRTUOverTCP {
		t.Fatalf("mode=%s", path.Mode)
	}
}

func TestResolveRegDataPathRejectTCPClient(t *testing.T) {
	p := regParam(t, "modbus")
	if err := p.SetField("work_mode", "tcp-client"); err != nil {
		t.Fatal(err)
	}
	if _, err := resolveRegDataPath(&p, &regOptions{mode: "auto"}); err == nil {
		t.Fatal("tcp-client should be rejected")
	}
}

func regParam(t *testing.T, appProto string) protocol.Param {
	t.Helper()
	var p protocol.Param
	kvs := map[string]string{
		"local_ip":   "192.168.15.42",
		"local_port": "502",
		"work_mode":  "tcp-server",
		"app_proto":  appProto,
	}
	for k, v := range kvs {
		if err := p.SetField(k, v); err != nil {
			t.Fatalf("SetField(%s): %v", k, err)
		}
	}
	return p
}
