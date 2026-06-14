package protocol

import "fmt"

// UDP 管理帧 = [0x5A 0x4C][cmd 1B][param 167B],共 170 字节。

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
// 要求参数区至少 ParamLen(167)字节,超出部分截断——这样读回的 Param 永远完整,
// 杜绝短包补 0 后被整块回写(0x02/0x07)清零 user_param 等未知区(防 review 指出的数据破坏)。
func DecodeUDP(b []byte) (UDPCmd, Param, error) {
	var p Param
	if len(b) < 3+ParamLen {
		return 0, p, fmt.Errorf("UDP 帧过短:%d 字节(参数区须 >= %d)", len(b), ParamLen)
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
