package transport

import (
	"context"
	"log"
	"net"
	"time"

	"zlan/internal/protocol"
)

const udpReadBuf = 2048 // 单包上限内,一次读一整包

// directedBroadcast 计算某网段的定向广播地址(ip | ^mask)。
// 仅处理 IPv4;入参非法时返回 nil。抽成纯函数以便单测。
func directedBroadcast(ip net.IP, mask net.IPMask) net.IP {
	ip4 := ip.To4()
	if ip4 == nil || len(mask) != net.IPv4len {
		return nil
	}
	bc := make(net.IP, net.IPv4len)
	for i := range net.IPv4len {
		bc[i] = ip4[i] | ^mask[i]
	}
	return bc
}

// broadcastTargets 返回应发送广播的目标 IP:每个 up 且支持广播的 IPv4 网卡的
// 定向广播地址,外加全局广播 255.255.255.255 兜底。多网卡场景下,255 只走默认
// 路由那张网卡,故必须逐网卡定向广播才能覆盖所有网段(SPEC §14)。
func broadcastTargets() []net.IP {
	var targets []net.IP
	if ifaces, err := net.Interfaces(); err == nil {
		for _, ifi := range ifaces {
			if ifi.Flags&net.FlagUp == 0 ||
				ifi.Flags&net.FlagBroadcast == 0 ||
				ifi.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, err := ifi.Addrs()
			if err != nil {
				continue // 单个网卡失败不致命
			}
			for _, a := range addrs {
				ipnet, ok := a.(*net.IPNet)
				if !ok {
					continue
				}
				if bc := directedBroadcast(ipnet.IP, ipnet.Mask); bc != nil {
					targets = append(targets, bc)
				}
			}
		}
	}
	return append(targets, net.IPv4bcast)
}

// Discover 广播发现局域网内所有设备。在 wait 窗口内逐网卡定向广播并反复收包,
// 只接受设备应答(0x01)帧、按 DevID 去重。找到 0 台不算错误(返回空列表)。
func Discover(ctx context.Context, wait time.Duration) ([]Device, error) {
	return DiscoverWithOptions(ctx, DiscoverOptions{Wait: wait})
}

// DiscoverOptions 控制广播发现的源地址与目标地址。Targets 为空时自动对所有
// up 且支持广播的 IPv4 网卡发定向广播,并追加 255.255.255.255 兜底。
type DiscoverOptions struct {
	Wait     time.Duration
	BindIP   net.IP
	BindPort int
	Targets  []net.IP
}

// DiscoverWithOptions 广播发现局域网内所有设备。
func DiscoverWithOptions(ctx context.Context, opt DiscoverOptions) ([]Device, error) {
	bindIP := opt.BindIP
	if bindIP == nil {
		bindIP = net.IPv4zero
	}
	targets := opt.Targets
	if len(targets) == 0 {
		targets = broadcastTargets()
	}

	// 绑 0.0.0.0:0:三平台都能正确收广播(Windows 绑具体 IP 收不到),源端口任取。
	// 设备应答回到"发广播的源端口"(SPEC §17.6),故无需绑 1092,也不与 monitor 抢端口。
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: bindIP, Port: opt.BindPort})
	if err != nil {
		return nil, classifyErr(err)
	}
	defer conn.Close()

	payload := protocol.EncodeBroadcastQuery()
	for _, t := range targets {
		dst := &net.UDPAddr{IP: t, Port: protocol.MgmtPort}
		if _, err := conn.WriteToUDP(payload, dst); err != nil {
			log.Printf("warn: 向 %v 发送广播失败: %v", dst, err) // 单目标失败不致命
		}
	}

	_ = conn.SetReadDeadline(time.Now().Add(opt.Wait))

	// 设好初始 deadline 后再启动取消监听,否则 ctx 提前取消时设的 deadline 会被上面这行覆盖。
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.SetReadDeadline(time.Now())
		case <-stop:
		}
	}()
	seen := make(map[[6]byte]bool)
	var devs []Device
	buf := make([]byte, udpReadBuf)
	for {
		n, src, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil {
				return devs, ctx.Err()
			}
			if isTimeout(err) {
				break // 窗口到期,正常结束(唯一预期退出路径)
			}
			return devs, classifyErr(err)
		}
		cmd, p, derr := protocol.DecodeUDP(buf[:n])
		if derr != nil || cmd != protocol.UDPCmdDeviceReply {
			continue // 丢弃 magic/类型不符的包,以及自己发出的查询包
		}
		id := p.DevID()
		if seen[id] {
			continue // 多网卡/多次广播可能收到同台设备多份应答
		}
		seen[id] = true
		addr := *src // ReadFromUDP 复用 src,需复制
		devs = append(devs, Device{Addr: &addr, Param: p})
	}
	return devs, nil
}

// UDPConn 是对单台设备的 UDP 单播连接(connected socket,内核自动过滤非对端来源)。
type UDPConn struct {
	conn    *net.UDPConn
	timeout time.Duration
	retries int
}

// Dial 建立到设备地址的 UDP 单播连接。局域网传 IP:1092;外网传设备上报的源地址。
func Dial(addr *net.UDPAddr, timeout time.Duration, retries int) (*UDPConn, error) {
	conn, err := net.DialUDP("udp4", nil, addr)
	if err != nil {
		return nil, classifyErr(err)
	}
	return &UDPConn{conn: conn, timeout: timeout, retries: retries}, nil
}

// Close 关闭连接。
func (c *UDPConn) Close() error { return c.conn.Close() }

// PersistentWriteReboots reflects the ZLAN UDP 0x02 semantics: save parameters
// and reboot. Sending an immediate read query can race the device while it is
// committing the write.
func (c *UDPConn) PersistentWriteReboots() bool { return true }

// ReadParam 发一对一查询(0x04),读回设备应答(0x01)的参数。
func (c *UDPConn) ReadParam() (protocol.Param, error) {
	var last error
	buf := make([]byte, udpReadBuf)
	payload := protocol.EncodeUnicastQuery()
	for attempt := 0; attempt <= c.retries; attempt++ {
		if _, err := c.conn.Write(payload); err != nil {
			last = classifyErr(err)
			continue
		}
		_ = c.conn.SetReadDeadline(time.Now().Add(c.timeout))
		n, err := c.conn.Read(buf)
		if err != nil {
			if isTimeout(err) {
				last = ErrNoDevice
				continue // 超时可重试
			}
			return protocol.Param{}, classifyErr(err)
		}
		cmd, p, derr := protocol.DecodeUDP(buf[:n])
		if derr != nil || cmd != protocol.UDPCmdDeviceReply {
			last = ErrNoDevice
			continue
		}
		return p, nil
	}
	return protocol.Param{}, last
}

// WriteParam 整块写回参数。UDP 不支持部分写,changed 仅作日志;按 mode 发 0x02
// (持久,设备重启)或 0x03(临时串口参数,不重启不保存)。UDP 改参无应答,发完即返回。
func (c *UDPConn) WriteParam(p protocol.Param, changed []string, mode WriteMode) error {
	cmd := protocol.UDPCmdModifyParam
	if mode == WriteVolatile {
		cmd = protocol.UDPCmdSetSerial
	}
	_, err := c.conn.Write(protocol.EncodeUDP(cmd, p))
	return classifyErr(err)
}

// Reboot 重启设备:读回参数(0x04)后原样以 0x02 回发触发重启(官方方法,SPEC §6)。
func (c *UDPConn) Reboot() error {
	p, err := c.ReadParam()
	if err != nil {
		return err
	}
	_, err = c.conn.Write(protocol.EncodeUDP(protocol.UDPCmdModifyParam, p))
	return classifyErr(err)
}

// MonitorFunc 处理一个入站包;返回非 nil 则原路回发给来源地址(用于外网设备改参)。
type MonitorFunc func(src *net.UDPAddr, cmd protocol.UDPCmd, p protocol.Param) []byte

// Monitor 被动监听设备周期上报(0x01),每个合法包交 handler 处理,可选回包。
// 绑 0.0.0.0:port,通过周期性短 deadline 醒来检查 ctx 以响应取消。
func Monitor(ctx context.Context, port int, handler MonitorFunc) error {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: net.IPv4zero, Port: port})
	if err != nil {
		return classifyErr(err)
	}
	defer conn.Close()

	buf := make([]byte, udpReadBuf)
	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		_ = conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
		n, src, err := conn.ReadFromUDP(buf)
		if err != nil {
			if isTimeout(err) {
				continue // 周期醒来检查 ctx,不是错误
			}
			return classifyErr(err)
		}
		cmd, p, derr := protocol.DecodeUDP(buf[:n])
		if derr != nil {
			continue
		}
		if reply := handler(src, cmd, p); reply != nil {
			if _, err := conn.WriteToUDP(reply, src); err != nil {
				log.Printf("warn: 回包给 %v 失败: %v", src, err)
			}
		}
	}
}
