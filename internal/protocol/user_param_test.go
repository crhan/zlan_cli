package protocol

import (
	"bytes"
	"testing"
)

func TestParseUserParamWiFiRev4Sample(t *testing.T) {
	raw := userParamRaw(t, "09 04 00 00 00 00 0a 04 00 00 00 00 02 04 74 65 73 74 03 01 44 05 01 86 04 03 6b 65 79 00")

	records, err := ParseUserParam(raw[:])
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 6 {
		t.Fatalf("records=%d", len(records))
	}
	cfg, err := WiFiConfigFromTLVs(records)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.HasSSID || cfg.SSID != "test" {
		t.Fatalf("ssid=%+v", cfg)
	}
	if !cfg.HasKey || cfg.Key != "key" {
		t.Fatalf("key=%+v", cfg)
	}
	if !cfg.HasChannel || cfg.Channel != 4 || !cfg.DHCPServerDisabled || cfg.EthWiFiBridge {
		t.Fatalf("channel bits=%+v", cfg)
	}
	if !cfg.HasModeCrypt || cfg.Mode != "sta" || cfg.Crypt != "auto" {
		t.Fatalf("mode/crypt=%+v", cfg)
	}
	counters, err := UserParamCountersFromTLVs(records)
	if err != nil {
		t.Fatal(err)
	}
	if !counters.HasSerialTxBytes || counters.SerialTxBytes != 0 ||
		!counters.HasSerialRxBytes || counters.SerialRxBytes != 0 {
		t.Fatalf("counters=%+v", counters)
	}
}

func TestUserParamCountersFromTLVs(t *testing.T) {
	counters, err := UserParamCountersFromTLVs([]UserTLV{
		{Type: UserTLVSerialTxBytes, Value: []byte{0x00, 0x00, 0x00, 0x10}},
		{Type: UserTLVSerialRxBytes, Value: []byte{0x00, 0x01, 0x40, 0x97}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !counters.HasSerialTxBytes || counters.SerialTxBytes != 16 {
		t.Fatalf("tx counter=%+v", counters)
	}
	if !counters.HasSerialRxBytes || counters.SerialRxBytes != 82071 {
		t.Fatalf("rx counter=%+v", counters)
	}

	_, err = UserParamCountersFromTLVs([]UserTLV{{Type: UserTLVSerialTxBytes, Value: []byte{1, 2, 3}}})
	if err == nil {
		t.Fatal("expected invalid counter length error")
	}
}

func TestReplaceWiFiConfigBuildsRev4APSample(t *testing.T) {
	raw := userParamRaw(t, "09 04 00 00 00 00 0a 04 00 00 00 00 02 04 74 65 73 74 03 01 44 05 01 86 04 03 6b 65 79 00")
	records, err := ParseUserParam(raw[:])
	if err != nil {
		t.Fatal(err)
	}
	next, err := ReplaceWiFiConfig(records, WiFiConfig{
		SSID:               "test2",
		Key:                "key",
		Channel:            4,
		DHCPServerDisabled: true,
		Mode:               "ap",
		Crypt:              "none",
		HasSSID:            true,
		HasKey:             true,
		HasChannel:         true,
		HasModeCrypt:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	got, err := BuildUserParam(next)
	if err != nil {
		t.Fatal(err)
	}
	want := userParamRaw(t, "09 04 00 00 00 00 0a 04 00 00 00 00 02 05 74 65 73 74 32 03 01 44 05 01 40 04 03 6b 65 79 00")
	if !bytes.Equal(got[:], want[:]) {
		t.Fatalf("got % x\nwant % x", got, want)
	}
}

func TestBuildUserParamRejectsOverflow(t *testing.T) {
	_, err := BuildUserParam([]UserTLV{{Type: UserTLVCustom, Value: make([]byte, UserParamLen-2)}})
	if err == nil {
		t.Fatal("expected overflow error because type 0 terminator would not fit")
	}
}

func userParamRaw(t *testing.T, prefix string) [UserParamLen]byte {
	t.Helper()
	var out [UserParamLen]byte
	raw := mustHex(t, prefix)
	if len(raw) > UserParamLen {
		t.Fatalf("prefix too long:%d", len(raw))
	}
	copy(out[:], raw)
	return out
}
