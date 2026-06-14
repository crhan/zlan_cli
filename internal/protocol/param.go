// Package protocol 实现 ZLAN(上海卓岚)联网模块的参数编解码与帧格式。
//
// 两条管理通道共享同一个 167 字节参数块(Param),但帧格式不同:
//   - UDP 管理端口(1092):整块收发,帧 = [5A 4C][cmd][167B]
//   - 串口命令模式:按 offset 部分读写,帧 = [ED..D7][cmd][pos][len][data]
//
// 权威字节布局、取值编码、待验证项见仓库根目录 SPEC.md。
package protocol

// ParamLen 是参数块固定长度。UDP 完整包 = 3(头) + ParamLen = 170 字节。
const ParamLen = 167

// Param 是 167 字节原始参数块。
//
// 设计:以原始字节为中心,而非逐字段强类型 struct。理由——
// 协议要求"读-改-写整块"(读回完整参数 → 改若干字段 → 整块写回),
// 原始字节天然保留未改字段(key/devid/user_param/只读位),零信息丢失;
// 且 get/set/info/json 全部由字段注册表(fields.go)派生,无 per-field 分支。
type Param [ParamLen]byte

// Clone 返回副本。"读-改-写"时在快照副本上修改,不污染原始读回数据。
func (p Param) Clone() Param { return p }

// Bytes 返回底层字节的切片视图(读用;修改请走 SetField)。
func (p *Param) Bytes() []byte { return p[:] }

// DevID 返回 6 字节设备唯一标识(MAC,offset 31)。DevID 不可改、每台唯一,
// 是设备的稳定主键;改参时若写入区间覆盖此处必须与设备匹配,否则设备丢弃全部写入。
func (p *Param) DevID() [6]byte {
	var d [6]byte
	copy(d[:], p[31:37])
	return d
}

// Connected 读连接状态(status@61 的 bit0)。1=TCP 已连接或处于 UDP 态。
// 注意:UDP 模式查询恒为已连接。
func (p *Param) Connected() bool { return p[61]&0x01 != 0 }

// --- UDP 通道常量(SPEC §1/§2.1)---

// MgmtPort 是 UDP 管理端口。
const MgmtPort = 1092

// UDP 帧 magic:'Z”L',线上固定 5A 4C(大端)。
const (
	udpMagic0 = 0x5A
	udpMagic1 = 0x4C
)

// UDPCmd 是 UDP 帧第 3 字节的命令类型。
type UDPCmd byte

const (
	UDPCmdBroadcastQuery UDPCmd = 0x00 // PC→设备(广播)查询所有设备
	UDPCmdDeviceReply    UDPCmd = 0x01 // 设备→PC 应答(也用于周期上报)
	UDPCmdModifyParam    UDPCmd = 0x02 // PC→设备 改参:必重启必存,参数区 DevID 须匹配
	UDPCmdSetSerial      UDPCmd = 0x03 // PC→设备 设串口参数:不重启不保存
	UDPCmdUnicastQuery   UDPCmd = 0x04 // PC→设备 一对一查询(单播)
)

// --- 串口通道常量(SPEC §1/§2.2)---

// serialMagic 是串口命令识别流,固定 10 字节。
var serialMagic = [10]byte{0xED, 0xF2, 0xA3, 0x56, 0xCA, 0xDB, 0x91, 0x84, 0xB0, 0xD7}

// SerialCmd 是串口帧第 11 字节的命令类型(bit0=1 写 / 0 读)。
type SerialCmd byte

const (
	SerialCmdRead        SerialCmd = 0x00 // 读参数
	SerialCmdWrite       SerialCmd = 0x01 // 写参数(不存;但 IP/掩码/网关/DHCP/DNS 改后自动重启并保存)
	SerialCmdWriteSave   SerialCmd = 0x03 // 写参数 + 存 Flash
	SerialCmdWriteReboot SerialCmd = 0x07 // 写参数 + 存 + 一定重启
)
