package protocol

import "fmt"

// UDP 管理帧 = [0x5A 0x4C][cmd 1B][param 167B],共 170 字节。

// minUDPParamLen 是可接受的最小参数区长度。文档称参数数量随版本不同,
// 但都大于 90 字节;故解码时放宽下限以兼容不同固件,超出 167 的截断、不足的补 0。
const minUDPParamLen = 90

// EncodeUDP 组装一个 UDP 管理帧(参数区固定 167 字节)。
func EncodeUDP(cmd UDPCmd, p Param) []byte {
	out := make([]byte, 3+ParamLen)
	out[0] = udpMagic0
	out[1] = udpMagic1
	out[2] = byte(cmd)
	copy(out[3:], p[:])
	return out
}

// DecodeUDP 校验 magic 与长度,返回命令类型与参数块。
// 参数区按实际长度拷贝进固定 167 字节:多余截断、不足补 0(兼容版本差异)。
func DecodeUDP(b []byte) (UDPCmd, Param, error) {
	var p Param
	if len(b) < 3+minUDPParamLen {
		return 0, p, fmt.Errorf("UDP 帧过短:%d 字节(至少需 %d)", len(b), 3+minUDPParamLen)
	}
	if b[0] != udpMagic0 || b[1] != udpMagic1 {
		return 0, p, fmt.Errorf("UDP 帧 magic 错误:期望 5A4C,实际 %02X%02X", b[0], b[1])
	}
	copy(p[:], b[3:])
	return UDPCmd(b[2]), p, nil
}

// EncodeBroadcastQuery 构造广播查询帧(0x00 + 全 0 参数)。
func EncodeBroadcastQuery() []byte { return EncodeUDP(UDPCmdBroadcastQuery, Param{}) }

// EncodeUnicastQuery 构造一对一查询帧(0x04 + 全 0 参数)。
func EncodeUnicastQuery() []byte { return EncodeUDP(UDPCmdUnicastQuery, Param{}) }
