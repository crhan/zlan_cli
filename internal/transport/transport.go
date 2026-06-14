// Package transport 实现 UDP 与串口两种设备管理通道的收发。
//
// 设计:device 层只依赖 Conn 接口(ReadParam/WriteParam/Reboot),不感知底层是
// UDP 单播还是串口直连。广播发现(Discover)与被动监听(Monitor)是 UDP 通道特有,
// 作为包级函数。错误统一翻译成用户可行动的语义错误(见下)。
package transport

import (
	"errors"
	"fmt"
	"net"
	"syscall"

	"zlan/internal/protocol"
)

// WriteMode 决定写操作的持久化/重启语义,由各通道映射到具体命令码。
type WriteMode int

const (
	// WritePersist 持久保存并使生效(UDP 0x02 必重启;串口 0x07 写存重启)。
	WritePersist WriteMode = iota
	// WriteVolatile 临时改串口参数,不保存不重启(UDP 0x03;串口 0x01)。
	WriteVolatile
)

// Device 是发现到的一台设备:应答源地址 + 完整参数快照。
// Addr 是设备应答的源地址;后续单播/外网改参必须复用它(SPEC §5),
// 不能凭设备 IP 重新构造 :1092——外网设备在 NAT 后,只有源地址能回得去。
type Device struct {
	Addr  *net.UDPAddr
	Param protocol.Param
}

// Conn 是对单台设备的管理连接(UDP 单播 或 串口直连)。
type Conn interface {
	// ReadParam 读回设备完整参数。
	ReadParam() (protocol.Param, error)
	// WriteParam 写回参数。changed 是改动字段名(串口据此切最小段,UDP 整块发送仅作日志)。
	WriteParam(p protocol.Param, changed []string, mode WriteMode) error
	// Reboot 重启设备。
	Reboot() error
	// Close 释放底层连接。
	Close() error
}

// 语义化错误:把底层网络错误翻译成用户可直接行动的信息。
var (
	ErrNoDevice    = errors.New("没有设备应答(检查设备是否上电、是否在同一局域网/可达)")
	ErrPermission  = errors.New("权限不足(绑定端口或发送广播被拒,尝试 sudo 或更换端口)")
	ErrUnreachable = errors.New("网络不可达(检查网卡/路由,目标网段是否可达)")
	ErrPortInUse   = errors.New("端口被占用(已有进程在监听该 UDP 端口)")
)

// classifyErr 把底层 errno / 超时翻译为语义错误;无法识别的原样返回(不吞错)。
func classifyErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, syscall.EACCES), errors.Is(err, syscall.EPERM):
		return fmt.Errorf("%w: %v", ErrPermission, err)
	case errors.Is(err, syscall.ENETUNREACH), errors.Is(err, syscall.EHOSTUNREACH):
		return fmt.Errorf("%w: %v", ErrUnreachable, err)
	case errors.Is(err, syscall.EADDRINUSE):
		return fmt.Errorf("%w: %v", ErrPortInUse, err)
	}
	if isTimeout(err) {
		return ErrNoDevice
	}
	return err
}

// isTimeout 判断错误是否为 I/O deadline 超时。
func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}
