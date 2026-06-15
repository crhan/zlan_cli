package cli

import (
	"net"
	"testing"
	"time"
)

func TestParseDiscoverOptions(t *testing.T) {
	opt, err := parseDiscoverOptions(
		3*time.Second,
		"192.168.1.10",
		12000,
		[]string{"192.168.1.255", "255.255.255.255"},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !opt.BindIP.Equal(net.ParseIP("192.168.1.10")) {
		t.Fatalf("bind=%v", opt.BindIP)
	}
	if opt.BindPort != 12000 {
		t.Fatalf("bind port=%d", opt.BindPort)
	}
	if len(opt.Targets) != 2 || !opt.Targets[0].Equal(net.ParseIP("192.168.1.255")) {
		t.Fatalf("targets=%v", opt.Targets)
	}
}

func TestParseDiscoverOptionsRejectsBadInput(t *testing.T) {
	if _, err := parseDiscoverOptions(time.Second, "not-ip", 0, nil); err == nil {
		t.Fatal("bad bind should fail")
	}
	if _, err := parseDiscoverOptions(time.Second, "", 70000, nil); err == nil {
		t.Fatal("bad port should fail")
	}
	if _, err := parseDiscoverOptions(time.Second, "", 0, []string{"not-ip"}); err == nil {
		t.Fatal("bad target should fail")
	}
}
