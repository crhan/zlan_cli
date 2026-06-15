package cli

import (
	"testing"

	"zlan/internal/device"
	"zlan/internal/protocol"
)

func TestParseImportArgsNetwork(t *testing.T) {
	host, overrides, err := parseImportArgs(&globalFlags{}, []string{
		"192.168.1.201",
		"local_ip=192.168.1.202",
	})
	if err != nil {
		t.Fatal(err)
	}
	if host != "192.168.1.201" {
		t.Fatalf("host=%q", host)
	}
	if overrides["local_ip"] != "192.168.1.202" {
		t.Fatalf("local_ip override=%q", overrides["local_ip"])
	}
}

func TestParseImportArgsSerial(t *testing.T) {
	host, overrides, err := parseImportArgs(&globalFlags{serial: "/dev/cu.fake"}, []string{"baud=9600"})
	if err != nil {
		t.Fatal(err)
	}
	if host != "" {
		t.Fatalf("serial host=%q", host)
	}
	if overrides["baud"] != "9600" {
		t.Fatalf("baud override=%q", overrides["baud"])
	}
}

func TestParseImportArgsValidation(t *testing.T) {
	if _, _, err := parseImportArgs(&globalFlags{}, nil); err == nil {
		t.Fatal("网络模式缺 target 应报错")
	}
	if _, _, err := parseImportArgs(&globalFlags{}, []string{"192.168.1.201", "bad"}); err == nil {
		t.Fatal("非法 override 应报错")
	}
}

func TestMarshalExportConfigCanBeParsed(t *testing.T) {
	var p protocol.Param
	copy(p[31:37], []byte{0x5a, 0x4c, 0x6f, 0x73, 0xcc, 0xd6})
	cfg := device.NewExportedConfig(p)

	for _, asJSON := range []bool{false, true} {
		data, err := marshalExportConfig(cfg, asJSON)
		if err != nil {
			t.Fatal(err)
		}
		got, _, err := device.ParseExportedConfig(data)
		if err != nil {
			t.Fatal(err)
		}
		if got != p {
			t.Fatal("导出的配置无法无损解析")
		}
	}
}
