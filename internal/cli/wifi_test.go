package cli

import (
	"testing"

	"zlan/internal/protocol"
)

func TestApplyWiFiAssignmentsPreservesPackedFields(t *testing.T) {
	base := protocol.WiFiConfig{
		SSID:               "old",
		Key:                "old-key",
		Channel:            6,
		DHCPServerDisabled: false,
		EthWiFiBridge:      true,
		Mode:               "sta",
		Crypt:              "auto",
		HasSSID:            true,
		HasKey:             true,
		HasChannel:         true,
		HasModeCrypt:       true,
	}
	got, changed, err := applyWiFiAssignments(base, map[string]string{
		"ssid":        "new",
		"dhcp_server": "disabled",
		"crypt":       "none",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.SSID != "new" || got.Channel != 6 || !got.DHCPServerDisabled || !got.EthWiFiBridge {
		t.Fatalf("channel packed fields not preserved: %+v", got)
	}
	if got.Mode != "sta" || got.Crypt != "none" {
		t.Fatalf("mode/crypt not preserved: %+v", got)
	}
	if !contains(changed, "ssid") || !contains(changed, "dhcp_server") || !contains(changed, "crypt") {
		t.Fatalf("changed=%v", changed)
	}
}

func TestApplyWiFiAssignmentsRequiresPackedBase(t *testing.T) {
	if _, _, err := applyWiFiAssignments(protocol.WiFiConfig{}, map[string]string{"bridge": "disabled"}); err == nil {
		t.Fatal("bridge without existing channel should fail")
	}
	if _, _, err := applyWiFiAssignments(protocol.WiFiConfig{}, map[string]string{"mode": "sta"}); err == nil {
		t.Fatal("mode without crypt should fail")
	}
}
