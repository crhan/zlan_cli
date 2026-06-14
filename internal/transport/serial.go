package transport

import (
	"fmt"
	"io"
	"time"

	"go.bug.st/serial"

	"zlan/internal/protocol"
)

// serialPort 抽象 go.bug.st/serial.Port 的最小子集,便于测试注入 fake。
type serialPort interface {
	io.ReadWriteCloser
	ResetInputBuffer() error
	SetReadTimeout(time.Duration) error
}

const (
	// defaultSerialChunk 是单帧读/写的最大段长。文档证实可一次写 104 字节,
	// 但 167 整块未演示(SPEC §17.2),故保守分段,避免触碰未知的单帧上限。
	defaultSerialChunk = 64
	// defaultSerialTimeout 是读满一段的总超时。
	defaultSerialTimeout = 2 * time.Second
	// defaultSerialSegDelay 是写多段之间的间隔(文档示例命令间需留时间)。
	defaultSerialSegDelay = 150 * time.Millisecond
	// serialReadTick 是单次 Read 的等待粒度(配合总超时轮询)。
	serialReadTick = 200 * time.Millisecond
)

// SerialConn 是通过串口命令模式管理一台直连设备的连接。
type SerialConn struct {
	port     serialPort
	chunk    int
	timeout  time.Duration
	segDelay time.Duration
}

// OpenSerial 按指定波特率打开串口。波特率必须与设备当前串口波特率一致,否则收不到应答。
func OpenSerial(portName string, baud int) (*SerialConn, error) {
	port, err := serial.Open(portName, &serial.Mode{BaudRate: baud})
	if err != nil {
		return nil, fmt.Errorf("打开串口 %s(波特率 %d)失败: %w", portName, baud, err)
	}
	if err := port.SetReadTimeout(serialReadTick); err != nil {
		_ = port.Close()
		return nil, fmt.Errorf("设置串口读超时失败: %w", err)
	}
	return &SerialConn{
		port:     port,
		chunk:    defaultSerialChunk,
		timeout:  defaultSerialTimeout,
		segDelay: defaultSerialSegDelay,
	}, nil
}

// Ports 列出本机可用串口设备。macOS 上建议选用 /dev/cu.* 而非 /dev/tty.*。
func Ports() ([]string, error) {
	ports, err := serial.GetPortsList()
	if err != nil {
		return nil, fmt.Errorf("枚举串口失败: %w", err)
	}
	return ports, nil
}

// Close 关闭串口。
func (c *SerialConn) Close() error { return c.port.Close() }

// ReadParam 分段读回完整 167 字节参数。
func (c *SerialConn) ReadParam() (protocol.Param, error) {
	var p protocol.Param
	for pos := 0; pos < protocol.ParamLen; pos += c.chunk {
		length := min(c.chunk, protocol.ParamLen-pos)
		data, err := c.readSegment(pos, length)
		if err != nil {
			return p, err
		}
		copy(p[pos:pos+length], data)
	}
	return p, nil
}

// WriteParam 按改动字段切最小段写回。每段再按 chunk 二次切分以尊重单帧上限。
// mode=持久 → 0x03(写存);临时 → 0x01(写不存)。
func (c *SerialConn) WriteParam(p protocol.Param, changed []string, mode WriteMode) error {
	cmd := protocol.SerialCmdWriteSave
	if mode == WriteVolatile {
		cmd = protocol.SerialCmdWrite
	}
	segs, err := protocol.ChangedSegments(&p, changed)
	if err != nil {
		return err
	}
	for _, seg := range segs {
		for off := 0; off < len(seg.Data); off += c.chunk {
			end := min(off+c.chunk, len(seg.Data))
			frame, err := protocol.EncodeSerialWrite(cmd, seg.Offset+off, seg.Data[off:end])
			if err != nil {
				return err
			}
			if err := c.writeFrame(frame); err != nil {
				return err
			}
			if c.segDelay > 0 {
				time.Sleep(c.segDelay)
			}
		}
	}
	return nil
}

// Reboot 重启设备:用 0x07 命令写 DevID 首字节(改不了,效果即重启),官方方法 SPEC §6/§3.7。
func (c *SerialConn) Reboot() error {
	frame, err := protocol.EncodeSerialWrite(protocol.SerialCmdWriteReboot, 31, []byte{0x00})
	if err != nil {
		return err
	}
	return c.writeFrame(frame)
}

// readSegment 发读命令并读回 length 字节裸数据。
func (c *SerialConn) readSegment(pos, length int) ([]byte, error) {
	frame, err := protocol.EncodeSerialRead(pos, length)
	if err != nil {
		return nil, err
	}
	if err := c.writeFrame(frame); err != nil {
		return nil, err
	}
	return c.readN(length)
}

// writeFrame 写命令前清空输入缓冲,避免上一次的残留字节污染本次读取。
func (c *SerialConn) writeFrame(b []byte) error {
	if err := c.port.ResetInputBuffer(); err != nil {
		return err
	}
	if _, err := c.port.Write(b); err != nil {
		return fmt.Errorf("串口写失败: %w", err)
	}
	return nil
}

// readN 在总超时内读满 n 字节(串口是流,数据可能分多次到达)。
func (c *SerialConn) readN(n int) ([]byte, error) {
	buf := make([]byte, n)
	deadline := time.Now().Add(c.timeout)
	got := 0
	for got < n {
		m, err := c.port.Read(buf[got:])
		if err != nil {
			return nil, fmt.Errorf("串口读失败: %w", err)
		}
		got += m
		if got < n && !time.Now().Before(deadline) {
			return nil, fmt.Errorf("串口读超时:期望 %d 字节,实际 %d(检查波特率是否与设备匹配)", n, got)
		}
	}
	return buf, nil
}

// 编译期断言:两种连接都满足 Conn 接口。
var (
	_ Conn = (*SerialConn)(nil)
	_ Conn = (*UDPConn)(nil)
)
