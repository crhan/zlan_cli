package transport

import (
	"context"
	"encoding/binary"
	"errors"
	"log"
	"net"
	"sync"
	"time"

	"zlan/internal/protocol"
)

const udpReadBuf = 2048 // 单包上限内,一次读一整包
const maxUnicastScanHosts = 512
const discoverSendWorkers = 64

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

type ipv4Network struct {
	ip   net.IP
	mask net.IPMask
}

type discoverProbe struct {
	bindIP           net.IP
	bindPort         int
	broadcastTargets []net.IP
	unicastTargets   []net.IP
	optional         bool
}

// localIPv4Networks 返回每个 up 且支持广播的非 loopback IPv4 地址/掩码。
// 同一网卡配置多个 IPv4 地址时每个地址都保留,discover 需要逐网段发广播。
func localIPv4Networks() []ipv4Network {
	var nets []ipv4Network
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
				ip4 := ipnet.IP.To4()
				if ip4 == nil || len(ipnet.Mask) != net.IPv4len {
					continue
				}
				nets = append(nets, ipv4Network{ip: cloneIP(ip4), mask: cloneMask(ipnet.Mask)})
			}
		}
	}
	return nets
}

// discoverProbes 生成广播探测计划。默认模式为每个本机 IPv4 网段开一个源地址
// socket,分别发该网段定向广播和 255.255.255.255。只用单个 0.0.0.0 socket 时,
// 多网段机器会受系统路由/源地址选择影响,可能只覆盖默认网段。
func discoverProbes(opt DiscoverOptions) []discoverProbe {
	return discoverProbesForNetworks(localIPv4Networks(), opt)
}

func discoverProbesForNetworks(nets []ipv4Network, opt DiscoverOptions) []discoverProbe {
	if len(opt.Targets) != 0 {
		bindIP := opt.BindIP
		if bindIP == nil {
			bindIP = net.IPv4zero
		}
		return []discoverProbe{{
			bindIP:           cloneIP(bindIP),
			bindPort:         opt.BindPort,
			broadcastTargets: uniqueIPs(opt.Targets),
		}}
	}

	var probes []discoverProbe
	for _, n := range nets {
		ip4 := n.ip.To4()
		if ip4 == nil {
			continue
		}
		if opt.BindIP != nil && !ip4.Equal(opt.BindIP.To4()) {
			continue
		}
		broadcastTargets := []net.IP{net.IPv4bcast}
		if bc := directedBroadcast(ip4, n.mask); bc != nil {
			broadcastTargets = append([]net.IP{bc}, broadcastTargets...)
		}
		probes = append(probes, discoverProbe{
			bindIP:           cloneIP(ip4),
			bindPort:         opt.BindPort,
			broadcastTargets: uniqueIPs(broadcastTargets),
			unicastTargets:   unicastScanTargets(ip4, n.mask),
			optional:         opt.BindIP == nil,
		})
	}
	if len(probes) != 0 {
		return probes
	}

	bindIP := opt.BindIP
	if bindIP == nil {
		bindIP = net.IPv4zero
	}
	return []discoverProbe{{
		bindIP:           cloneIP(bindIP),
		bindPort:         opt.BindPort,
		broadcastTargets: []net.IP{net.IPv4bcast},
	}}
}

func unicastScanTargets(ip net.IP, mask net.IPMask) []net.IP {
	ip4 := ip.To4()
	ones, bits := mask.Size()
	if ip4 == nil || bits != 32 || ones >= 31 {
		return nil
	}
	hostCount := (uint64(1) << uint(32-ones)) - 2
	if hostCount > maxUnicastScanHosts {
		return nil
	}
	ipNum := binary.BigEndian.Uint32(ip4)
	maskNum := binary.BigEndian.Uint32(mask)
	network := ipNum & maskNum
	broadcast := network | ^maskNum

	out := make([]net.IP, 0, int(hostCount))
	for n := network + 1; n < broadcast; n++ {
		if n == ipNum {
			continue
		}
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], n)
		out = append(out, cloneIP(net.IP(b[:])))
	}
	return preferHostOctet(out, 200)
}

func preferHostOctet(ips []net.IP, octet byte) []net.IP {
	preferred := make([]net.IP, 0, len(ips))
	rest := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		ip4 := ip.To4()
		if ip4 != nil && ip4[3] == octet {
			preferred = append(preferred, ip)
			continue
		}
		rest = append(rest, ip)
	}
	return append(preferred, rest...)
}

func uniqueIPs(ips []net.IP) []net.IP {
	seen := make(map[string]bool, len(ips))
	out := make([]net.IP, 0, len(ips))
	for _, ip := range ips {
		ip4 := ip.To4()
		if ip4 == nil {
			continue
		}
		key := ip4.String()
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, cloneIP(ip4))
	}
	return out
}

func cloneIP(ip net.IP) net.IP {
	ip4 := ip.To4()
	if ip4 == nil {
		return nil
	}
	out := make(net.IP, net.IPv4len)
	copy(out, ip4)
	return out
}

func cloneMask(mask net.IPMask) net.IPMask {
	out := make(net.IPMask, len(mask))
	copy(out, mask)
	return out
}

// Discover 发现局域网内所有设备。在 wait 窗口内逐网段广播并补充小网段单播探测,
// 只接受设备应答(0x01)帧、按 DevID 去重。找到 0 台不算错误(返回空列表)。
func Discover(ctx context.Context, wait time.Duration) ([]Device, error) {
	return DiscoverWithOptions(ctx, DiscoverOptions{Wait: wait})
}

// DiscoverOptions 控制发现的源地址与广播目标地址。Targets 为空时自动对所有
// up 且支持广播的 IPv4 网段发定向广播,追加 255.255.255.255 兜底,并对小网段单播探测。
type DiscoverOptions struct {
	Wait     time.Duration
	BindIP   net.IP
	BindPort int
	Targets  []net.IP
}

// DiscoverWithOptions 发现局域网内所有设备。
func DiscoverWithOptions(ctx context.Context, opt DiscoverOptions) ([]Device, error) {
	probes := discoverProbes(opt)
	conns := make([]discoverConn, 0, len(probes))
	var firstErr error
	for _, p := range probes {
		conn, err := net.ListenUDP("udp4", &net.UDPAddr{IP: p.bindIP, Port: p.bindPort})
		if err != nil {
			if !p.optional {
				return nil, classifyErr(err)
			}
			if firstErr == nil {
				firstErr = err
			}
			log.Printf("warn: 绑定 discover 源地址 %v:%d 失败: %v", p.bindIP, p.bindPort, err)
			continue
		}
		defer conn.Close()
		conns = append(conns, discoverConn{
			conn:             conn,
			broadcastTargets: p.broadcastTargets,
			unicastTargets:   p.unicastTargets,
		})
	}
	if len(conns) == 0 {
		return nil, classifyErr(firstErr)
	}

	deadline := time.Now().Add(opt.Wait)
	for _, c := range conns {
		_ = c.conn.SetReadDeadline(deadline)
	}

	// 设好初始 deadline 后再启动取消监听,否则 ctx 提前取消时设的 deadline 会被上面这行覆盖。
	stop := make(chan struct{})
	defer close(stop)
	go func() {
		select {
		case <-ctx.Done():
			for _, c := range conns {
				_ = c.conn.SetReadDeadline(time.Now())
			}
		case <-stop:
		}
	}()

	results := make(chan discoverResult)
	var wg sync.WaitGroup
	for _, c := range conns {
		wg.Add(1)
		go readDiscoverConn(c.conn, results, &wg)
	}
	go func() {
		wg.Wait()
		close(results)
	}()
	sendDone := sendDiscoverPackets(conns)

	seen := make(map[[6]byte]bool)
	var devs []Device
	var readErr error
	for res := range results {
		if res.err != nil {
			if ctx.Err() != nil {
				if readErr == nil {
					readErr = ctx.Err()
				}
				continue
			}
			if readErr == nil {
				readErr = classifyErr(res.err)
			}
			continue
		}
		id := res.dev.Param.DevID()
		if seen[id] {
			continue // 多网卡/多次广播可能收到同台设备多份应答
		}
		seen[id] = true
		devs = append(devs, res.dev)
	}
	for _, c := range conns {
		_ = c.conn.Close()
	}
	<-sendDone
	if ctx.Err() != nil {
		return devs, ctx.Err()
	}
	if len(devs) == 0 && readErr != nil {
		return nil, readErr
	}
	return devs, nil
}

type discoverConn struct {
	conn             *net.UDPConn
	broadcastTargets []net.IP
	unicastTargets   []net.IP
}

type discoverResult struct {
	dev Device
	err error
}

type discoverPacket struct {
	conn  *net.UDPConn
	dst   *net.UDPAddr
	data  []byte
	label string
}

func sendDiscoverPackets(conns []discoverConn) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		defer close(done)
		packets := discoverPackets(conns)
		if len(packets) == 0 {
			return
		}

		workers := min(discoverSendWorkers, len(packets))
		jobs := make(chan discoverPacket)
		var wg sync.WaitGroup
		for range workers {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for p := range jobs {
					if _, err := p.conn.WriteToUDP(p.data, p.dst); err != nil {
						if errors.Is(err, net.ErrClosed) {
							continue
						}
						log.Printf("warn: 向 %v 发送%s失败: %v", p.dst, p.label, err)
					}
				}
			}()
		}
		for _, p := range packets {
			jobs <- p
		}
		close(jobs)
		wg.Wait()
	}()
	return done
}

func discoverPackets(conns []discoverConn) []discoverPacket {
	broadcastPayload := protocol.EncodeBroadcastQuery()
	unicastPayload := protocol.EncodeUnicastQuery()
	var packets []discoverPacket
	for _, c := range conns {
		for _, t := range c.broadcastTargets {
			packets = append(packets, discoverPacket{
				conn:  c.conn,
				dst:   &net.UDPAddr{IP: t, Port: protocol.MgmtPort},
				data:  broadcastPayload,
				label: "广播",
			})
		}
	}
	for _, c := range conns {
		for _, t := range c.unicastTargets {
			packets = append(packets, discoverPacket{
				conn:  c.conn,
				dst:   &net.UDPAddr{IP: t, Port: protocol.MgmtPort},
				data:  unicastPayload,
				label: "单播探测",
			})
		}
	}
	return packets
}

func readDiscoverConn(conn *net.UDPConn, results chan<- discoverResult, wg *sync.WaitGroup) {
	defer wg.Done()
	buf := make([]byte, udpReadBuf)
	for {
		n, src, err := conn.ReadFromUDP(buf)
		if err != nil {
			if !isTimeout(err) {
				results <- discoverResult{err: err}
			}
			return
		}
		cmd, p, derr := protocol.DecodeUDP(buf[:n])
		if derr != nil || cmd != protocol.UDPCmdDeviceReply {
			continue // 丢弃 magic/类型不符的包,以及自己发出的查询包
		}
		addr := *src // ReadFromUDP 复用 src,需复制
		results <- discoverResult{dev: Device{Addr: &addr, Param: p}}
	}
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
