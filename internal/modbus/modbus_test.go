package modbus

import (
	"encoding/binary"
	"io"
	"net"
	"testing"
	"time"
)

func TestCRCAppendGolden(t *testing.T) {
	frame := []byte{0x01, 0x03, 0x00, 0x01, 0x00, 0x07}
	got := AppendCRC(frame)
	want := []byte{0x01, 0x03, 0x00, 0x01, 0x00, 0x07, 0x55, 0xc8}
	if string(got) != string(want) {
		t.Fatalf("crc frame=% x, want % x", got, want)
	}
	if !ValidCRC(got) {
		t.Fatalf("ValidCRC returned false")
	}
}

func TestReadRegistersModbusTCP(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	errc := make(chan error, 1)
	go func() {
		req := make([]byte, 12)
		if _, err := io.ReadFull(serverConn, req); err != nil {
			errc <- err
			return
		}
		tid := binary.BigEndian.Uint16(req[0:])
		if req[6] != 0x0b || req[7] != FuncReadHoldingRegisters {
			t.Errorf("request=% x", req)
		}
		resp := []byte{
			byte(tid >> 8), byte(tid),
			0x00, 0x00,
			0x00, 0x07,
			0x0b,
			FuncReadHoldingRegisters, 0x04,
			0x12, 0x34,
			0xab, 0xcd,
		}
		_, err := serverConn.Write(resp)
		errc <- err
	}()

	c := NewClient(clientConn, ModeTCP, time.Second)
	values, err := c.ReadRegisters(0x0b, FuncReadHoldingRegisters, 0x0001, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 2 || values[0] != 0x1234 || values[1] != 0xabcd {
		t.Fatalf("values=%#v", values)
	}
	if err := <-errc; err != nil {
		t.Fatal(err)
	}
}

func TestReadRegistersModbusTCPSkipsUnrelatedTransaction(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	errc := make(chan error, 1)
	go func() {
		req := make([]byte, 12)
		if _, err := io.ReadFull(serverConn, req); err != nil {
			errc <- err
			return
		}
		tid := binary.BigEndian.Uint16(req[0:])
		if tid != 139 {
			t.Errorf("tid=%d, want 139", tid)
		}

		foreign := []byte{
			0x00, 0x01,
			0x00, 0x00,
			0x00, 0x06,
			0x0d,
			FuncWriteSingleRegister,
			0x00, 0x02,
			0x00, 0x02,
		}
		resp := []byte{
			byte(tid >> 8), byte(tid),
			0x00, 0x00,
			0x00, 0x05,
			0x0d,
			FuncReadHoldingRegisters, 0x02,
			0x00, 0xf6,
		}
		if _, err := serverConn.Write(append(foreign, resp...)); err != nil {
			errc <- err
			return
		}
		errc <- nil
	}()

	c := NewClient(clientConn, ModeTCP, time.Second)
	c.nextTID = 138
	values, err := c.ReadRegisters(0x0d, FuncReadHoldingRegisters, 0x0000, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(values) != 1 || values[0] != 0x00f6 {
		t.Fatalf("values=%#v", values)
	}
	if err := <-errc; err != nil {
		t.Fatal(err)
	}
}

func TestWriteRegistersRTUOverTCP(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()

	errc := make(chan error, 1)
	go func() {
		req := make([]byte, 8)
		if _, err := io.ReadFull(serverConn, req); err != nil {
			errc <- err
			return
		}
		if !ValidCRC(req) {
			t.Errorf("bad request crc: % x", req)
		}
		if req[0] != 0x01 || req[1] != FuncWriteSingleRegister {
			t.Errorf("request=% x", req)
		}
		_, err := serverConn.Write(req)
		errc <- err
	}()

	c := NewClient(clientConn, ModeRTUOverTCP, time.Second)
	if err := c.WriteRegisters(0x01, 0x0002, []uint16{0x000b}); err != nil {
		t.Fatal(err)
	}
	if err := <-errc; err != nil {
		t.Fatal(err)
	}
}
