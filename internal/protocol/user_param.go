package protocol

import (
	"encoding/binary"
	"fmt"
	"strconv"
	"strings"
)

const (
	UserParamOffset = 115
	UserParamLen    = 52
)

const (
	UserTLVEnd              byte = 0
	UserTLVFunctionSelect   byte = 1
	UserTLVWiFiSSID         byte = 2
	UserTLVWiFiChannel      byte = 3
	UserTLVWiFiPassword     byte = 4
	UserTLVWiFiModeCrypt    byte = 5
	UserTLVMultiDestIP      byte = 6
	UserTLVProxyServer      byte = 8
	UserTLVSerialTxBytes    byte = 9
	UserTLVSerialRxBytes    byte = 10
	UserTLVCustom           byte = 255
	wifiChannelDHCPDisabled byte = 0x40
	wifiChannelEthBridge    byte = 0x80
	wifiModeAP              byte = 0x40
	wifiModeSTA             byte = 0x80
	wifiCryptMask           byte = 0x3f
)

// UserTLV is one record in user_param@115, documented by UDP Rev.4.
type UserTLV struct {
	Type  byte
	Value []byte
}

// UserParamCounters is the status-like subset stored in user_param TLVs.
type UserParamCounters struct {
	SerialTxBytes    uint32
	SerialRxBytes    uint32
	HasSerialTxBytes bool
	HasSerialRxBytes bool
}

// WiFiConfig is the WiFi subset stored inside user_param TLVs.
type WiFiConfig struct {
	SSID string
	Key  string

	Channel            int
	DHCPServerDisabled bool
	EthWiFiBridge      bool

	Mode  string // "ap" or "sta"
	Crypt string // "none", "wep64", "tkip", "aes", "wep128", "auto", or unknown(n) when reading

	HasSSID      bool
	HasKey       bool
	HasChannel   bool
	HasModeCrypt bool
}

// UserTLVs parses the fixed 52-byte user_param region from a Param.
func (p *Param) UserTLVs() ([]UserTLV, error) {
	return ParseUserParam(p[UserParamOffset : UserParamOffset+UserParamLen])
}

// SetUserTLVs rebuilds the fixed 52-byte user_param region in a Param.
func (p *Param) SetUserTLVs(records []UserTLV) error {
	raw, err := BuildUserParam(records)
	if err != nil {
		return err
	}
	copy(p[UserParamOffset:UserParamOffset+UserParamLen], raw[:])
	return nil
}

// ParseUserParam parses Rev.4 user_param TLV bytes until type 0.
func ParseUserParam(raw []byte) ([]UserTLV, error) {
	if len(raw) != UserParamLen {
		return nil, fmt.Errorf("user_param 长度错误:期望 %d 字节,实际 %d 字节", UserParamLen, len(raw))
	}
	var out []UserTLV
	for i := 0; i < len(raw); {
		typ := raw[i]
		i++
		if typ == UserTLVEnd {
			return out, nil
		}
		if i >= len(raw) {
			return nil, fmt.Errorf("user_param TLV type 0x%02x 缺少长度", typ)
		}
		n := int(raw[i])
		i++
		if i+n > len(raw) {
			return nil, fmt.Errorf("user_param TLV type 0x%02x 长度 %d 越界", typ, n)
		}
		val := make([]byte, n)
		copy(val, raw[i:i+n])
		out = append(out, UserTLV{Type: typ, Value: val})
		i += n
	}
	return nil, fmt.Errorf("user_param 缺少 type 0 结束标记")
}

// BuildUserParam builds the fixed 52-byte user_param region and appends type 0.
func BuildUserParam(records []UserTLV) ([UserParamLen]byte, error) {
	var out [UserParamLen]byte
	pos := 0
	for _, rec := range records {
		if rec.Type == UserTLVEnd {
			return out, fmt.Errorf("user_param 记录中不能显式包含 type 0")
		}
		if len(rec.Value) > 255 {
			return out, fmt.Errorf("user_param TLV type 0x%02x 超长:%d", rec.Type, len(rec.Value))
		}
		if pos+2+len(rec.Value) >= UserParamLen {
			return out, fmt.Errorf("user_param TLV 总长度超过 %d 字节", UserParamLen)
		}
		out[pos] = rec.Type
		out[pos+1] = byte(len(rec.Value))
		copy(out[pos+2:], rec.Value)
		pos += 2 + len(rec.Value)
	}
	out[pos] = UserTLVEnd
	return out, nil
}

// UserParamCountersFromTLVs decodes status counters documented by UDP Rev.4.
func UserParamCountersFromTLVs(records []UserTLV) (UserParamCounters, error) {
	var out UserParamCounters
	for _, rec := range records {
		switch rec.Type {
		case UserTLVSerialTxBytes:
			v, err := userTLVUint32(rec, "serial tx bytes")
			if err != nil {
				return out, err
			}
			out.SerialTxBytes = v
			out.HasSerialTxBytes = true
		case UserTLVSerialRxBytes:
			v, err := userTLVUint32(rec, "serial rx bytes")
			if err != nil {
				return out, err
			}
			out.SerialRxBytes = v
			out.HasSerialRxBytes = true
		}
	}
	return out, nil
}

// UserTLVName returns the documented Rev.4 TLV name when known.
func UserTLVName(typ byte) string {
	switch typ {
	case UserTLVEnd:
		return "end"
	case UserTLVFunctionSelect:
		return "function_select"
	case UserTLVWiFiSSID:
		return "wifi_ssid"
	case UserTLVWiFiChannel:
		return "wifi_channel"
	case UserTLVWiFiPassword:
		return "wifi_password"
	case UserTLVWiFiModeCrypt:
		return "wifi_mode_crypt"
	case UserTLVMultiDestIP:
		return "multi_dest_ip"
	case UserTLVProxyServer:
		return "proxy_server"
	case UserTLVSerialTxBytes:
		return "serial_tx_bytes"
	case UserTLVSerialRxBytes:
		return "serial_rx_bytes"
	case UserTLVCustom:
		return "custom"
	default:
		return "unknown"
	}
}

// WiFiConfigFromTLVs decodes WiFi-related records. Duplicate WiFi TLVs use last-one-wins.
func WiFiConfigFromTLVs(records []UserTLV) (WiFiConfig, error) {
	var cfg WiFiConfig
	for _, rec := range records {
		switch rec.Type {
		case UserTLVWiFiSSID:
			cfg.SSID = string(rec.Value)
			cfg.HasSSID = true
		case UserTLVWiFiPassword:
			cfg.Key = string(rec.Value)
			cfg.HasKey = true
		case UserTLVWiFiChannel:
			if len(rec.Value) != 1 {
				return cfg, fmt.Errorf("wifi channel TLV 长度应为 1,实际 %d", len(rec.Value))
			}
			v := rec.Value[0]
			cfg.Channel = int(v & 0x0f)
			cfg.DHCPServerDisabled = v&wifiChannelDHCPDisabled != 0
			cfg.EthWiFiBridge = v&wifiChannelEthBridge != 0
			cfg.HasChannel = true
		case UserTLVWiFiModeCrypt:
			if len(rec.Value) != 1 {
				return cfg, fmt.Errorf("wifi mode/crypt TLV 长度应为 1,实际 %d", len(rec.Value))
			}
			v := rec.Value[0]
			ap := v&wifiModeAP != 0
			sta := v&wifiModeSTA != 0
			switch {
			case ap && sta:
				return cfg, fmt.Errorf("wifi mode 同时标记 AP 和 STA:0x%02x", v)
			case ap:
				cfg.Mode = "ap"
			case sta:
				cfg.Mode = "sta"
			default:
				cfg.Mode = ""
			}
			cfg.Crypt = wifiCryptName(v & wifiCryptMask)
			cfg.HasModeCrypt = true
		}
	}
	return cfg, nil
}

// ReplaceWiFiConfig removes existing WiFi TLVs and appends the supplied WiFi records.
func ReplaceWiFiConfig(records []UserTLV, cfg WiFiConfig) ([]UserTLV, error) {
	out := make([]UserTLV, 0, len(records)+4)
	for _, rec := range records {
		if rec.Type >= UserTLVWiFiSSID && rec.Type <= UserTLVWiFiModeCrypt {
			continue
		}
		out = append(out, cloneTLV(rec))
	}
	wifi, err := wifiTLVs(cfg)
	if err != nil {
		return nil, err
	}
	out = append(out, wifi...)
	if _, err := BuildUserParam(out); err != nil {
		return nil, err
	}
	return out, nil
}

func wifiTLVs(cfg WiFiConfig) ([]UserTLV, error) {
	out := make([]UserTLV, 0, 4)
	if cfg.HasSSID {
		out = append(out, UserTLV{Type: UserTLVWiFiSSID, Value: []byte(cfg.SSID)})
	}
	if cfg.HasChannel {
		if cfg.Channel < 1 || cfg.Channel > 11 {
			return nil, fmt.Errorf("wifi channel 必须在 1..11")
		}
		v := byte(cfg.Channel)
		if cfg.DHCPServerDisabled {
			v |= wifiChannelDHCPDisabled
		}
		if cfg.EthWiFiBridge {
			v |= wifiChannelEthBridge
		}
		out = append(out, UserTLV{Type: UserTLVWiFiChannel, Value: []byte{v}})
	}
	if cfg.HasModeCrypt {
		mode, err := wifiModeValue(cfg.Mode)
		if err != nil {
			return nil, err
		}
		crypt, err := wifiCryptValue(cfg.Crypt)
		if err != nil {
			return nil, err
		}
		out = append(out, UserTLV{Type: UserTLVWiFiModeCrypt, Value: []byte{mode | crypt}})
	}
	if cfg.HasKey {
		out = append(out, UserTLV{Type: UserTLVWiFiPassword, Value: []byte(cfg.Key)})
	}
	return out, nil
}

func cloneTLV(rec UserTLV) UserTLV {
	val := make([]byte, len(rec.Value))
	copy(val, rec.Value)
	return UserTLV{Type: rec.Type, Value: val}
}

func userTLVUint32(rec UserTLV, name string) (uint32, error) {
	if len(rec.Value) != 4 {
		return 0, fmt.Errorf("%s TLV 长度应为 4,实际 %d", name, len(rec.Value))
	}
	return binary.BigEndian.Uint32(rec.Value), nil
}

func wifiModeValue(s string) (byte, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "ap":
		return wifiModeAP, nil
	case "sta", "station":
		return wifiModeSTA, nil
	}
	return 0, fmt.Errorf("wifi mode 非法取值 %q,可选:ap | sta", s)
}

func wifiCryptValue(s string) (byte, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "none", "open":
		return 0, nil
	case "wep64":
		return 1, nil
	case "tkip":
		return 2, nil
	case "aes":
		return 4, nil
	case "wep128":
		return 5, nil
	case "auto":
		return 6, nil
	}
	return 0, fmt.Errorf("wifi crypt 非法取值 %q,可选:none | wep64 | tkip | aes | wep128 | auto", s)
}

func wifiCryptName(v byte) string {
	switch v {
	case 0:
		return "none"
	case 1:
		return "wep64"
	case 2:
		return "tkip"
	case 4:
		return "aes"
	case 5:
		return "wep128"
	case 6:
		return "auto"
	default:
		return "unknown(" + strconv.Itoa(int(v)) + ")"
	}
}
