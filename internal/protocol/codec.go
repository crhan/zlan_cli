package protocol

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
)

// Unmarshal 从字节构造 Param,校验长度必须正好 ParamLen。
func Unmarshal(b []byte) (Param, error) {
	var p Param
	if len(b) != ParamLen {
		return p, fmt.Errorf("参数长度错误:期望 %d 字节,实际 %d 字节", ParamLen, len(b))
	}
	copy(p[:], b)
	return p, nil
}

// GetField 读取字段(含位域子字段),返回人类可读字符串。
func (p *Param) GetField(name string) (string, error) {
	if bf, ok := bitFieldIndex[name]; ok {
		return strconv.Itoa(int((p[bf.Byte] >> bf.Bit) & 1)), nil
	}
	f, ok := fieldIndex[name]
	if !ok {
		return "", fmt.Errorf("未知字段:%s", name)
	}
	return p.format(f), nil
}

// SetField 写入字段(含位域子字段)。只读字段、保留区拒绝写入。
func (p *Param) SetField(name, value string) error {
	if bf, ok := bitFieldIndex[name]; ok {
		if bf.ReadOnly {
			return fmt.Errorf("字段只读,不可设置:%s", name)
		}
		v, err := parseBit(value)
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if v {
			p[bf.Byte] |= 1 << bf.Bit
		} else {
			p[bf.Byte] &^= 1 << bf.Bit
		}
		return nil
	}
	f, ok := fieldIndex[name]
	if !ok {
		return fmt.Errorf("未知字段:%s", name)
	}
	if f.ReadOnly {
		return fmt.Errorf("字段只读,不可设置:%s", name)
	}
	return p.parseInto(f, value)
}

// format 按字段类型把原始字节格式化为可读字符串。
func (p *Param) format(f Field) string {
	b := p[f.Offset : f.Offset+f.Size]
	switch f.Kind {
	case KindIPv4:
		return net.IPv4(b[0], b[1], b[2], b[3]).String()
	case KindU8:
		return strconv.Itoa(int(b[0]))
	case KindU16BE:
		return strconv.Itoa(int(binary.BigEndian.Uint16(b)))
	case KindEnum:
		if n, ok := f.Enum.byVal[b[0]]; ok {
			return n
		}
		return fmt.Sprintf("unknown(%d)", b[0])
	case KindCString:
		raw := trimZero(b)
		if f.Name == "dev_name" {
			return decodeDeviceName(raw)
		}
		return string(raw)
	case KindMAC:
		return net.HardwareAddr(b).String()
	case KindVersion:
		return fmt.Sprintf("1.%d", 383+int(b[0]))
	case KindBitByte:
		return fmt.Sprintf("0x%02x", b[0])
	case KindRaw, KindOpaque:
		return hex.EncodeToString(b)
	}
	return hex.EncodeToString(b)
}

// parseInto 校验并把 value 写入字段对应的字节区间。
func (p *Param) parseInto(f Field, value string) error {
	dst := p[f.Offset : f.Offset+f.Size]
	switch f.Kind {
	case KindIPv4:
		ip := net.ParseIP(strings.TrimSpace(value)).To4()
		if ip == nil {
			return fmt.Errorf("%s:非法 IPv4 地址 %q", f.Name, value)
		}
		if f.Name == "group_ip" && (ip[0] < 224 || ip[0] > 239) {
			return fmt.Errorf("group_ip 必须在 224.0.0.0~239.255.255.255 范围内")
		}
		copy(dst, ip)
	case KindU8:
		n, err := parseUint(value, 8)
		if err != nil {
			return fmt.Errorf("%s:需 0..255 整数,得到 %q", f.Name, value)
		}
		dst[0] = byte(n)
	case KindU16BE:
		n, err := parseUint(value, 16)
		if err != nil {
			return fmt.Errorf("%s:需 0..65535 整数,得到 %q", f.Name, value)
		}
		if f.Name == "packing_len" && (n < 1 || n > 1400) {
			return fmt.Errorf("packing_len 需在 1..1400")
		}
		binary.BigEndian.PutUint16(dst, uint16(n))
	case KindEnum:
		v, ok := f.Enum.byName[strings.ToLower(strings.TrimSpace(value))]
		if !ok {
			return fmt.Errorf("%s:非法取值 %q,可选:%s", f.Name, value, strings.Join(f.Enum.options(), " | "))
		}
		dst[0] = v
	case KindCString:
		if strings.ContainsRune(value, 0) {
			return fmt.Errorf("%s:不能含 NUL 字节", f.Name)
		}
		bs, err := encodeCString(f, value)
		if err != nil {
			return err
		}
		if len(bs) > f.Size-1 { // 须留至少 1 字节给结尾 0
			return fmt.Errorf("%s:字符串过长,最多 %d 字节(含结尾 0)", f.Name, f.Size)
		}
		for i := range dst {
			dst[i] = 0 // 清旧残留,保证结尾 0
		}
		copy(dst, bs)
	case KindBitByte:
		n, err := parseUint(value, 8)
		if err != nil {
			return fmt.Errorf("%s:需 0..255 或 0x.. 字节值,得到 %q", f.Name, value)
		}
		dst[0] = byte(n)
	case KindRaw:
		raw, err := decodeHex(value)
		if err != nil {
			return fmt.Errorf("%s:需 hex 字符串:%w", f.Name, err)
		}
		if len(raw) > f.Size {
			return fmt.Errorf("%s:超长,最多 %d 字节", f.Name, f.Size)
		}
		// 只覆盖用户给出的字节,保留尾部不动(SPEC §3:key 是密码,勿清零)
		copy(dst[:len(raw)], raw)
	case KindOpaque:
		return fmt.Errorf("%s 为保留区,不支持直接设置", f.Name)
	default:
		return fmt.Errorf("%s:该字段类型不支持设置", f.Name)
	}
	return nil
}

// trimZero 截断到首个 0 字节(C 字符串语义)。
func trimZero(b []byte) []byte {
	before, _, _ := bytes.Cut(b, []byte{0})
	return before
}

// decodeDeviceName 兼容厂家工具写入的 GBK/ANSI 中文名:优先按 UTF-8,
// 失败后按 GBK 解码,再失败才做替换字符兜底。
func decodeDeviceName(b []byte) string {
	if utf8.Valid(b) {
		return string(b)
	}
	decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(b)
	if err == nil && utf8.Valid(decoded) {
		return string(decoded)
	}
	return string(bytes.ToValidUTF8(b, []byte("?")))
}

// encodeCString 编码定长 C 字符串。dev_name 使用 GBK,以兼容 ZLVircom/厂家工具;
// 其他字符串字段保持原始 UTF-8/ASCII 字节语义。
func encodeCString(f Field, value string) ([]byte, error) {
	if f.Name != "dev_name" {
		return []byte(value), nil
	}
	encoded, err := simplifiedchinese.GBK.NewEncoder().Bytes([]byte(value))
	if err != nil {
		return nil, fmt.Errorf("%s:需可按 GBK 编码:%w", f.Name, err)
	}
	return encoded, nil
}

// parseBit 解析位值,接受 0/1/true/false/on/off/yes/no。
func parseBit(s string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "1", "true", "on", "yes":
		return true, nil
	case "0", "false", "off", "no":
		return false, nil
	}
	return false, fmt.Errorf("位值需 0/1/true/false,得到 %q", s)
}

// parseUint 解析无符号整数,支持 0x 前缀的十六进制;bits 限定位宽。
func parseUint(s string, bits int) (uint64, error) {
	s = strings.TrimSpace(s)
	base := 10
	if l := strings.ToLower(s); strings.HasPrefix(l, "0x") {
		base = 16
		s = s[2:]
	}
	return strconv.ParseUint(s, base, bits)
}

// decodeHex 解析 hex 字符串(允许 0x 前缀);不做长度填充,长度由调用方校验。
func decodeHex(s string) ([]byte, error) {
	s = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(s)), "0x")
	return hex.DecodeString(s)
}
