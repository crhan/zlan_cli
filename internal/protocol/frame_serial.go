package protocol

import "fmt"

// 串口命令模式帧 = [10B magic][cmd][pos][len][data...]。
// 读命令无 data;写命令 data 长度 == len。读响应是 len 字节裸数据(无帧头),
// 由调用方读取,不在此解析。

// EncodeSerialRead 构造串口读命令(从 pos 读 length 字节)。
func EncodeSerialRead(pos, length int) ([]byte, error) {
	if err := checkSeg(pos, length); err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(serialMagic)+3)
	out = append(out, serialMagic[:]...)
	out = append(out, byte(SerialCmdRead), byte(pos), byte(length))
	return out, nil
}

// EncodeSerialWrite 构造串口写命令。cmd 取 SerialCmdWrite/WriteSave/WriteReboot。
func EncodeSerialWrite(cmd SerialCmd, pos int, data []byte) ([]byte, error) {
	if err := checkSeg(pos, len(data)); err != nil {
		return nil, err
	}
	out := make([]byte, 0, len(serialMagic)+3+len(data))
	out = append(out, serialMagic[:]...)
	out = append(out, byte(cmd), byte(pos), byte(len(data)))
	out = append(out, data...)
	return out, nil
}

// checkSeg 校验读写区间不越界,且长度可放进 1 字节 len 字段。
func checkSeg(pos, length int) error {
	if pos < 0 || length < 0 || pos+length > ParamLen {
		return fmt.Errorf("串口读写区间越界:pos=%d len=%d(参数块 %d 字节)", pos, length, ParamLen)
	}
	if length > 255 {
		return fmt.Errorf("串口单次长度 %d 超过 255(len 为 1 字节)", length)
	}
	return nil
}
