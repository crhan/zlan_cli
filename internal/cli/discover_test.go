package cli

import (
	"net"
	"testing"
	"time"

	"zlan/internal/transport"
)

func TestFormatProbeLine(t *testing.T) {
	cases := []struct {
		name string
		info transport.ProbeInfo
		want string
	}{
		{
			name: "网卡+定向广播+单播",
			info: transport.ProbeInfo{
				Iface:            "eth0",
				LocalIP:          net.ParseIP("192.168.1.10"),
				LocalPort:        54321,
				BroadcastTargets: []net.IP{net.ParseIP("192.168.1.255"), net.IPv4bcast},
				UnicastTargets:   253,
			},
			want: "  出口 [eth0] 192.168.1.10:54321 → 广播 192.168.1.255, 255.255.255.255(单播探测 253)",
		},
		{
			name: "大网段无单播",
			info: transport.ProbeInfo{
				Iface:            "docker0",
				LocalIP:          net.ParseIP("172.17.0.1"),
				LocalPort:        40000,
				BroadcastTargets: []net.IP{net.ParseIP("172.17.255.255"), net.IPv4bcast},
			},
			want: "  出口 [docker0] 172.17.0.1:40000 → 广播 172.17.255.255, 255.255.255.255",
		},
		{
			name: "显式 target 内核选源",
			info: transport.ProbeInfo{
				LocalIP:          net.IPv4zero,
				LocalPort:        53986,
				BroadcastTargets: []net.IP{net.ParseIP("192.168.2.255")},
			},
			want: "  出口 0.0.0.0:53986 → 广播 192.168.2.255",
		},
	}
	for _, tc := range cases {
		if got := formatProbeLine(tc.info); got != tc.want {
			t.Errorf("%s:\n got %q\nwant %q", tc.name, got, tc.want)
		}
	}
}

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
