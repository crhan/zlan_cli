// Package mqtt implements the MQTT 3.1.1 QoS 0 subset needed by zlan.
package mqtt

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// Config contains the connection parameters for an MQTT broker.
type Config struct {
	Addr     string
	ClientID string
	Username string
	Password string
	Timeout  time.Duration
}

// Client is a minimal MQTT 3.1.1 client that can publish QoS 0 messages.
type Client struct {
	conn    net.Conn
	timeout time.Duration
}

// Dial opens a TCP connection and sends an MQTT CONNECT packet.
func Dial(ctx context.Context, cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.Addr) == "" {
		return nil, fmt.Errorf("mqtt broker address is required")
	}
	if strings.TrimSpace(cfg.ClientID) == "" {
		return nil, fmt.Errorf("mqtt client id is required")
	}
	timeout := cfg.Timeout
	dialer := net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", cfg.Addr)
	if err != nil {
		return nil, err
	}
	c := &Client{conn: conn, timeout: timeout}
	if err := c.connect(cfg); err != nil {
		_ = conn.Close()
		return nil, err
	}
	return c, nil
}

// Close sends DISCONNECT and closes the underlying TCP connection.
func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	_ = c.withDeadline()
	_, _ = c.conn.Write([]byte{0xe0, 0x00})
	return c.conn.Close()
}

// Publish publishes payload to topic using QoS 0.
func (c *Client) Publish(topic string, payload []byte, retain bool) error {
	if err := validateTopic(topic); err != nil {
		return err
	}
	body := make([]byte, 0, 2+len(topic)+len(payload))
	body = appendString(body, topic)
	body = append(body, payload...)

	header := byte(0x30)
	if retain {
		header |= 0x01
	}
	packet := append([]byte{header}, encodeRemainingLength(len(body))...)
	packet = append(packet, body...)

	if err := c.withDeadline(); err != nil {
		return err
	}
	_, err := c.conn.Write(packet)
	return err
}

func (c *Client) connect(cfg Config) error {
	flags := byte(0x02) // clean session
	if cfg.Password != "" {
		flags |= 0x40
	}
	if cfg.Username != "" || cfg.Password != "" {
		flags |= 0x80
	}

	var body []byte
	body = appendString(body, "MQTT")
	body = append(body, 0x04, flags, 0x00, 0x3c) // MQTT 3.1.1, keepalive 60s
	body = appendString(body, cfg.ClientID)
	if cfg.Username != "" || cfg.Password != "" {
		body = appendString(body, cfg.Username)
	}
	if cfg.Password != "" {
		body = appendString(body, cfg.Password)
	}

	packet := append([]byte{0x10}, encodeRemainingLength(len(body))...)
	packet = append(packet, body...)

	if err := c.withDeadline(); err != nil {
		return err
	}
	if _, err := c.conn.Write(packet); err != nil {
		return err
	}

	resp := make([]byte, 4)
	if _, err := io.ReadFull(c.conn, resp); err != nil {
		return err
	}
	if resp[0] != 0x20 || resp[1] != 0x02 {
		return fmt.Errorf("unexpected mqtt connack header: % x", resp)
	}
	if resp[3] != 0 {
		return fmt.Errorf("mqtt connack rejected connection, code=%d", resp[3])
	}
	return nil
}

func (c *Client) withDeadline() error {
	if c.timeout <= 0 {
		return nil
	}
	return c.conn.SetDeadline(time.Now().Add(c.timeout))
}

func validateTopic(topic string) error {
	if strings.TrimSpace(topic) == "" {
		return fmt.Errorf("mqtt topic is required")
	}
	if strings.ContainsAny(topic, "+#") {
		return fmt.Errorf("publish topic must not contain MQTT wildcards")
	}
	if len(topic) > 65535 {
		return fmt.Errorf("mqtt topic is too long")
	}
	return nil
}

func appendString(dst []byte, s string) []byte {
	if len(s) > 65535 {
		panic("mqtt string too long")
	}
	var b [2]byte
	binary.BigEndian.PutUint16(b[:], uint16(len(s)))
	dst = append(dst, b[:]...)
	return append(dst, s...)
}

func encodeRemainingLength(n int) []byte {
	if n < 0 || n > 268435455 {
		panic("invalid mqtt remaining length")
	}
	var out []byte
	for {
		digit := byte(n % 128)
		n /= 128
		if n > 0 {
			digit |= 0x80
		}
		out = append(out, digit)
		if n == 0 {
			return out
		}
	}
}
