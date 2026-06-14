// Package device 编排 protocol 与 transport,提供面向命令行的高层设备操作。
package device

import (
	"context"
	"fmt"
	"net"
	"time"

	"zlan/internal/protocol"
	"zlan/internal/transport"
)

// Target 描述要操作的设备及连接参数。
type Target struct {
	Serial  string        // 串口路径;非空走串口通道,此时 Host 应为空
	Baud    int           // 串口波特率(须匹配设备当前波特率)
	Host    string        // IP 或 DevID(MAC);UDP 通道用
	Timeout time.Duration // 单次操作/发现超时
	Retries int           // 单播重试次数
}

// Endpoint 是已解析、已连接的目标。
type Endpoint struct {
	Conn transport.Conn
	// Snapshot 在"按 DevID 广播匹配"时顺带带回,供 set 复用,避免重复 discover。可能为 nil。
	Snapshot *protocol.Param
	DevID    string
}

// Open 解析 Target 并建立连接。三种寻址:串口直连 / UDP 单播到 IP / 按 DevID 广播匹配。
func (t Target) Open(ctx context.Context) (*Endpoint, error) {
	if t.Serial != "" {
		c, err := transport.OpenSerial(t.Serial, t.Baud)
		if err != nil {
			return nil, err
		}
		return &Endpoint{Conn: c}, nil
	}
	if t.Host == "" {
		return nil, fmt.Errorf("缺少目标:给出设备 IP 或 DevID(MAC),或用 --serial 走串口")
	}
	if ip := net.ParseIP(t.Host); ip != nil {
		c, err := transport.Dial(&net.UDPAddr{IP: ip, Port: protocol.MgmtPort}, t.Timeout, t.Retries)
		if err != nil {
			return nil, err
		}
		return &Endpoint{Conn: c, DevID: t.Host}, nil
	}
	// 既非 IP,当作 DevID(MAC),广播匹配出其真实地址。
	want, err := parseDevID(t.Host)
	if err != nil {
		return nil, fmt.Errorf("目标 %q 既不是 IP 也不是合法 DevID(MAC)", t.Host)
	}
	devs, err := transport.Discover(ctx, t.Timeout)
	if err != nil {
		return nil, err
	}
	for i := range devs {
		if devs[i].Param.DevID() == want {
			c, err := transport.Dial(devs[i].Addr, t.Timeout, t.Retries)
			if err != nil {
				return nil, err
			}
			snap := devs[i].Param
			return &Endpoint{Conn: c, Snapshot: &snap, DevID: t.Host}, nil
		}
	}
	return nil, fmt.Errorf("局域网未发现 DevID 为 %s 的设备", t.Host)
}

// parseDevID 把 MAC 字符串解析为 6 字节 DevID。
func parseDevID(s string) ([6]byte, error) {
	mac, err := net.ParseMAC(s)
	if err != nil || len(mac) != 6 {
		return [6]byte{}, fmt.Errorf("非法 DevID(需 6 字节 MAC):%s", s)
	}
	return [6]byte(mac), nil
}
