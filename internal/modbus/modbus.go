// Package modbus implements the small Modbus subset needed for direct register
// access through a ZLAN data channel.
package modbus

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// Mode describes the wire protocol spoken on the ZLAN data TCP connection.
type Mode string

const (
	// ModeTCP sends Modbus TCP ADUs. Use this when ZLAN app_proto=modbus.
	ModeTCP Mode = "modbus-tcp"
	// ModeRTUOverTCP sends Modbus RTU frames, including CRC, over the TCP stream.
	// Use this when ZLAN app_proto=transparent and the serial slave speaks RTU.
	ModeRTUOverTCP Mode = "rtu-over-tcp"
)

const (
	FuncReadHoldingRegisters byte = 0x03
	FuncReadInputRegisters   byte = 0x04
	FuncWriteSingleRegister  byte = 0x06
	FuncWriteMultipleRegs    byte = 0x10
)

// ErrTCPDesync marks a Modbus TCP stream that cannot be safely parsed further.
// Callers that keep long-lived TCP sessions should reconnect before retrying.
var ErrTCPDesync = errors.New("modbus tcp stream desynchronized")

type tcpDesyncError struct {
	msg string
}

func (e tcpDesyncError) Error() string { return e.msg }
func (e tcpDesyncError) Unwrap() error { return ErrTCPDesync }

// Client talks Modbus over an already selected ZLAN data-channel mode.
type Client struct {
	conn    net.Conn
	mode    Mode
	timeout time.Duration
	nextTID uint16
}

// DialTCP opens a TCP data-channel connection.
func DialTCP(ctx context.Context, addr string, mode Mode, timeout time.Duration) (*Client, error) {
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, err
	}
	return NewClient(conn, mode, timeout), nil
}

// NewClient wraps conn. The caller owns Close through the returned client.
func NewClient(conn net.Conn, mode Mode, timeout time.Duration) *Client {
	return &Client{conn: conn, mode: mode, timeout: timeout}
}

// Close closes the underlying connection.
func (c *Client) Close() error { return c.conn.Close() }

// ReadRegisters reads holding or input registers with function 0x03/0x04.
func (c *Client) ReadRegisters(unit byte, fn byte, addr, count uint16) ([]uint16, error) {
	if fn != FuncReadHoldingRegisters && fn != FuncReadInputRegisters {
		return nil, fmt.Errorf("unsupported read function 0x%02x", fn)
	}
	if count == 0 || count > 125 {
		return nil, fmt.Errorf("register count must be 1..125")
	}
	pdu := []byte{fn, byte(addr >> 8), byte(addr), byte(count >> 8), byte(count)}
	resp, err := c.roundTrip(unit, pdu)
	if err != nil {
		return nil, err
	}
	if err := checkException(resp, fn); err != nil {
		return nil, err
	}
	if len(resp) < 2 || resp[0] != fn {
		return nil, fmt.Errorf("unexpected read response: % x", resp)
	}
	byteCount := int(resp[1])
	if byteCount != int(count)*2 || len(resp) != 2+byteCount {
		return nil, fmt.Errorf("bad read response length: % x", resp)
	}
	values := make([]uint16, count)
	for i := range values {
		values[i] = binary.BigEndian.Uint16(resp[2+i*2:])
	}
	return values, nil
}

// WriteRegisters writes one or more holding registers. A single value uses
// function 0x06; multiple values use function 0x10.
func (c *Client) WriteRegisters(unit byte, addr uint16, values []uint16) error {
	if len(values) == 0 {
		return fmt.Errorf("at least one register value is required")
	}
	if len(values) > 123 {
		return fmt.Errorf("at most 123 registers can be written at once")
	}
	if len(values) == 1 {
		value := values[0]
		pdu := []byte{
			FuncWriteSingleRegister,
			byte(addr >> 8), byte(addr),
			byte(value >> 8), byte(value),
		}
		resp, err := c.roundTrip(unit, pdu)
		if err != nil {
			return err
		}
		if err := checkException(resp, FuncWriteSingleRegister); err != nil {
			return err
		}
		if len(resp) != len(pdu) || string(resp) != string(pdu) {
			return fmt.Errorf("unexpected write response: % x", resp)
		}
		return nil
	}

	quantity := uint16(len(values))
	pdu := make([]byte, 6+len(values)*2)
	pdu[0] = FuncWriteMultipleRegs
	binary.BigEndian.PutUint16(pdu[1:], addr)
	binary.BigEndian.PutUint16(pdu[3:], quantity)
	pdu[5] = byte(len(values) * 2)
	for i, value := range values {
		binary.BigEndian.PutUint16(pdu[6+i*2:], value)
	}
	resp, err := c.roundTrip(unit, pdu)
	if err != nil {
		return err
	}
	if err := checkException(resp, FuncWriteMultipleRegs); err != nil {
		return err
	}
	if len(resp) != 5 || resp[0] != FuncWriteMultipleRegs ||
		binary.BigEndian.Uint16(resp[1:]) != addr ||
		binary.BigEndian.Uint16(resp[3:]) != quantity {
		return fmt.Errorf("unexpected write response: % x", resp)
	}
	return nil
}

func (c *Client) roundTrip(unit byte, pdu []byte) ([]byte, error) {
	if c.timeout > 0 {
		_ = c.conn.SetDeadline(time.Now().Add(c.timeout))
	}
	switch c.mode {
	case ModeTCP:
		return c.roundTripTCP(unit, pdu)
	case ModeRTUOverTCP:
		return c.roundTripRTUOverTCP(unit, pdu)
	default:
		return nil, fmt.Errorf("unsupported modbus mode %q", c.mode)
	}
}

func (c *Client) roundTripTCP(unit byte, pdu []byte) ([]byte, error) {
	c.nextTID++
	tid := c.nextTID
	adu := make([]byte, 7+len(pdu))
	binary.BigEndian.PutUint16(adu[0:], tid)
	binary.BigEndian.PutUint16(adu[2:], 0)
	binary.BigEndian.PutUint16(adu[4:], uint16(1+len(pdu)))
	adu[6] = unit
	copy(adu[7:], pdu)
	if _, err := c.conn.Write(adu); err != nil {
		return nil, err
	}

	skipped := 0
	for {
		header := make([]byte, 7)
		if _, err := io.ReadFull(c.conn, header); err != nil {
			return nil, err
		}
		gotTID := binary.BigEndian.Uint16(header[0:])
		proto := binary.BigEndian.Uint16(header[2:])
		length := int(binary.BigEndian.Uint16(header[4:]))
		if length < 2 || length > 254 {
			return nil, tcpDesyncError{msg: fmt.Sprintf("bad modbus tcp length %d", length)}
		}
		resp := make([]byte, length-1)
		if _, err := io.ReadFull(c.conn, resp); err != nil {
			return nil, err
		}
		if proto != 0 {
			return nil, tcpDesyncError{msg: fmt.Sprintf("unexpected protocol id %d", proto)}
		}
		if gotTID != tid {
			skipped++
			if skipped > 16 {
				return nil, tcpDesyncError{msg: fmt.Sprintf("too many unrelated modbus tcp responses while waiting for transaction id %d", tid)}
			}
			continue
		}
		if header[6] != unit {
			return nil, fmt.Errorf("unexpected unit id %d, want %d", header[6], unit)
		}
		return resp, nil
	}
}

func (c *Client) roundTripRTUOverTCP(unit byte, pdu []byte) ([]byte, error) {
	frame := append([]byte{unit}, pdu...)
	frame = AppendCRC(frame)
	if _, err := c.conn.Write(frame); err != nil {
		return nil, err
	}

	head := make([]byte, 2)
	if _, err := io.ReadFull(c.conn, head); err != nil {
		return nil, err
	}
	if head[0] != unit {
		return nil, fmt.Errorf("unexpected unit id %d, want %d", head[0], unit)
	}

	var rest []byte
	fn := head[1]
	switch {
	case fn == pdu[0]|0x80:
		rest = make([]byte, 3) // exception code + CRC
	case fn == FuncReadHoldingRegisters || fn == FuncReadInputRegisters:
		count := make([]byte, 1)
		if _, err := io.ReadFull(c.conn, count); err != nil {
			return nil, err
		}
		rest = append(rest, count[0])
		rest = append(rest, make([]byte, int(count[0])+2)...)
		if _, err := io.ReadFull(c.conn, rest[1:]); err != nil {
			return nil, err
		}
		rsp := append(head, rest...)
		if !ValidCRC(rsp) {
			return nil, fmt.Errorf("bad rtu crc: % x", rsp)
		}
		return rsp[1 : len(rsp)-2], nil
	case fn == FuncWriteSingleRegister || fn == FuncWriteMultipleRegs:
		rest = make([]byte, 6) // address + value/count + CRC
	default:
		return nil, fmt.Errorf("unexpected rtu function 0x%02x", fn)
	}
	if len(rest) > 0 {
		if _, err := io.ReadFull(c.conn, rest); err != nil {
			return nil, err
		}
	}
	rsp := append(head, rest...)
	if !ValidCRC(rsp) {
		return nil, fmt.Errorf("bad rtu crc: % x", rsp)
	}
	return rsp[1 : len(rsp)-2], nil
}

func checkException(resp []byte, wantFn byte) error {
	if len(resp) >= 2 && resp[0] == wantFn|0x80 {
		return fmt.Errorf("modbus exception 0x%02x", resp[1])
	}
	return nil
}

// CRC returns the Modbus RTU CRC16 value. When serialized, low byte is written
// before high byte.
func CRC(data []byte) uint16 {
	var crc uint16 = 0xffff
	for _, b := range data {
		crc ^= uint16(b)
		for range 8 {
			if crc&1 != 0 {
				crc = (crc >> 1) ^ 0xa001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

// AppendCRC returns a copy of frame with Modbus RTU CRC bytes appended.
func AppendCRC(frame []byte) []byte {
	out := make([]byte, len(frame)+2)
	copy(out, frame)
	crc := CRC(frame)
	out[len(frame)] = byte(crc)
	out[len(frame)+1] = byte(crc >> 8)
	return out
}

// ValidCRC reports whether frame ends with a valid Modbus RTU CRC.
func ValidCRC(frame []byte) bool {
	if len(frame) < 3 {
		return false
	}
	want := CRC(frame[:len(frame)-2])
	got := uint16(frame[len(frame)-2]) | uint16(frame[len(frame)-1])<<8
	return got == want
}
