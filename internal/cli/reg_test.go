package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/spf13/cobra"

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

func TestParseReadKindCoil(t *testing.T) {
	got, err := parseReadKind("coil")
	if err != nil {
		t.Fatal(err)
	}
	if got.fn != modbus.FuncReadCoils || got.kind != "coil" || !got.bits || got.maxCount != 2000 {
		t.Fatalf("kind=%+v", got)
	}
}

func TestParseWriteKindCoil(t *testing.T) {
	kind, coil, err := parseWriteKind("coil")
	if err != nil {
		t.Fatal(err)
	}
	if kind != "coil" || !coil {
		t.Fatalf("kind=%s coil=%v", kind, coil)
	}
}

func TestParseCoilValue(t *testing.T) {
	for _, raw := range []string{"on", "true", "1", "yes"} {
		got, err := parseCoilValue(raw)
		if err != nil || !got {
			t.Fatalf("%s => %v %v", raw, got, err)
		}
	}
	for _, raw := range []string{"off", "false", "0", "no"} {
		got, err := parseCoilValue(raw)
		if err != nil || got {
			t.Fatalf("%s => %v %v", raw, got, err)
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

func TestEmitRegPollSampleText(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	emitRegPollSample(cmd, &globalFlags{}, pollSpecForTest(), []uint16{12, 34})

	got := out.String()
	if !strings.Contains(got, "unit=1 holding") ||
		!strings.Contains(got, "0x0010=12") ||
		!strings.Contains(got, "0x0011=34") {
		t.Fatalf("poll text output=%q", got)
	}
}

func TestEmitRegPollSampleJSONLine(t *testing.T) {
	var out bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&out)

	emitRegPollSample(cmd, &globalFlags{jsonOut: true}, pollSpecForTest(), []uint16{12, 34})

	got := out.String()
	if strings.Count(got, "\n") != 1 {
		t.Fatalf("poll json should be one line, got %q", got)
	}
	var obj regResultJSON
	if err := json.Unmarshal(out.Bytes(), &obj); err != nil {
		t.Fatalf("invalid poll json: %v", err)
	}
	if obj.Target != "192.168.1.200" || obj.Address != 0x0010 || obj.Count != 2 || len(obj.Values) != 2 {
		t.Fatalf("poll json object=%+v", obj)
	}
	if obj.Values[0].Value != 12 || obj.Values[1].Value != 34 {
		t.Fatalf("poll json values=%+v", obj.Values)
	}
}

func pollSpecForTest() *regPollSpec {
	return &regPollSpec{
		host: "192.168.1.200",
		path: regDataPath{
			Address:  "192.168.1.200:502",
			Mode:     modbus.ModeTCP,
			WorkMode: "tcp-server",
			AppProto: "modbus",
		},
		unit:  1,
		fn:    modbus.FuncReadHoldingRegisters,
		kind:  "holding",
		addr:  0x0010,
		count: 2,
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
