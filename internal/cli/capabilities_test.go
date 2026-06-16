package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"zlan/internal/protocol"
)

func TestCapabilityRowFromParam(t *testing.T) {
	p := capabilityParam(t, "ZL5143DI", "192.168.15.47", 0xbe, 0x07)
	row := capabilityRowFromParam(&p)

	if row.DevID != "28:8b:65:da:74:bf" || row.Name != "ZL5143DI" || row.IP != "192.168.15.47" {
		t.Fatalf("identity row=%+v", row)
	}
	if row.FuncSel != "0xbe" || row.FuncSel2 != "0x07" {
		t.Fatalf("func bytes row=%+v", row)
	}
	for _, want := range []string{"dns", "realcom", "modbus_tcp_to_rtu", "io_config", "udp_multicast", "multi_target_ip"} {
		if !contains(row.Supported, want) {
			t.Fatalf("supported=%v missing %s", row.Supported, want)
		}
	}
	if row.UnknownBits != nil {
		t.Fatalf("unknown bits=%v, want nil", row.UnknownBits)
	}
}

func TestCapabilityUnknownFuncSel2Bits(t *testing.T) {
	p := capabilityParam(t, "1401新风", "192.168.15.42", 0xbe, 0x46)
	row := capabilityRowFromParam(&p)

	if row.UnknownBits["func_sel2"] != "bit6" {
		t.Fatalf("unknown bits=%v", row.UnknownBits)
	}
	if !contains(row.Supported, "udp_multicast") || !contains(row.Supported, "multi_target_ip") {
		t.Fatalf("supported=%v", row.Supported)
	}
	if contains(row.Supported, "io_config") {
		t.Fatalf("io_config should be false: %v", row.Supported)
	}
}

func TestRenderCapabilitiesText(t *testing.T) {
	rows := []capabilityRow{
		capabilityRowFromParamPtr(t, "ZL5143DI", "192.168.15.47", 0xbe, 0x07),
		capabilityRowFromParamPtr(t, "1401新风", "192.168.15.42", 0xbe, 0x46),
	}
	var buf bytes.Buffer
	if err := renderCapabilities(&buf, rows, false); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "ZL5143DI") ||
		!strings.Contains(got, "io_config") ||
		!strings.Contains(got, "func_sel2=bit6") {
		t.Fatalf("text output=%q", got)
	}
}

func TestRenderCapabilitiesJSON(t *testing.T) {
	rows := []capabilityRow{
		capabilityRowFromParamPtr(t, "ZL5143DI", "192.168.15.47", 0xbe, 0x07),
	}
	var buf bytes.Buffer
	if err := renderCapabilities(&buf, rows, true); err != nil {
		t.Fatal(err)
	}
	var out []capabilityJSON
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || !out[0].Capabilities["io_config"] || out[0].ModelGuess != "unknown" {
		t.Fatalf("json=%+v", out)
	}
}

func capabilityRowFromParamPtr(t *testing.T, name, ip string, funcSel, funcSel2 byte) capabilityRow {
	t.Helper()
	p := capabilityParam(t, name, ip, funcSel, funcSel2)
	return capabilityRowFromParam(&p)
}

func capabilityParam(t *testing.T, name, ip string, funcSel, funcSel2 byte) protocol.Param {
	t.Helper()
	var p protocol.Param
	copy(p[31:37], []byte{0x28, 0x8b, 0x65, 0xda, 0x74, 0xbf})
	if err := p.SetField("dev_name", name); err != nil {
		t.Fatal(err)
	}
	if err := p.SetField("local_ip", ip); err != nil {
		t.Fatal(err)
	}
	p[103] = 255 // ver 1.638
	p[104] = funcSel
	p[112] = funcSel2
	return p
}

func contains(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
