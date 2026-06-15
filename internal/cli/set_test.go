package cli

import "testing"

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
