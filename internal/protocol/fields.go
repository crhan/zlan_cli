package protocol

import "sort"

// Kind 是字段的取值类型,决定 codec 如何格式化/解析该字段。
type Kind uint8

const (
	KindIPv4    Kind = iota // 4 字节大端 IP
	KindU8                  // 单字节无符号整数
	KindU16BE               // 2 字节大端无符号整数
	KindEnum                // 单字节枚举(查表,见 Field.Enum)
	KindCString             // 以 0 结尾的可见字符串(定长区,写时补 0 清尾)
	KindMAC                 // 6 字节 MAC(冒号分隔展示)
	KindVersion             // 单字节版本(展示为 1.(383+v))
	KindBitByte             // 位域容器字节(展示为 hex,位含义见 BitField)
	KindRaw                 // 定长原始字节(hex 展示/解析,如 key)
	KindOpaque              // 纯透传保留区,不解析、禁止直接设置
)

// Field 是参数块里一个字段的注册项。get/set/info/json 全部由此表派生。
type Field struct {
	Name     string     // 机器名,set k=v 的 k 与 get 的 field 都用它
	Offset   int        // 参数块内字节偏移
	Size     int        // 字节数
	Kind     Kind       // 取值类型
	Enum     *enumTable // KindEnum 专用
	ReadOnly bool       // 只读字段:SetField 拒绝写入(读-改-写时保持原值回写)
	Group    string     // info 分组
	Desc     string     // 人类可读说明
}

// BitField 是位域容器字节里的一个子位,机器名形如 "func_en.need_password"。
type BitField struct {
	Name     string
	Parent   string
	Byte     int   // 容器字节偏移
	Bit      uint8 // 位序(0..7)
	ReadOnly bool
	Desc     string
}

// enumTable 是单字节枚举的双向映射。
type enumTable struct {
	byVal  map[byte]string
	byName map[string]byte
}

func newEnum(byVal map[byte]string) *enumTable {
	e := &enumTable{byVal: byVal, byName: make(map[string]byte, len(byVal))}
	for v, n := range byVal {
		e.byName[n] = v
	}
	return e
}

// options 返回按取值排序的名字列表(用于错误提示/补全)。
func (e *enumTable) options() []string {
	vals := make([]int, 0, len(e.byVal))
	for v := range e.byVal {
		vals = append(vals, int(v))
	}
	sort.Ints(vals)
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		out = append(out, e.byVal[byte(v)])
	}
	return out
}

// 枚举表(SPEC §3 表 A/B 及各字段编码)。
var (
	enumWorkMode = newEnum(map[byte]string{
		0: "tcp-server", 1: "tcp-client", 2: "udp", 3: "udp-multicast",
	})
	// baud 索引含非标准的 7200(index 3),必须查表,不能用线性公式。
	enumBaud = newEnum(map[byte]string{
		0: "1200", 1: "2400", 2: "4800", 3: "7200", 4: "9600", 5: "14400",
		6: "19200", 7: "28800", 8: "38400", 9: "57600", 10: "76800",
		11: "115200", 12: "230400", 13: "460800",
	})
	// parity:两份文档对 1/2(even/odd)定义相反,此处暂用 UDP 文档。待真机验证(SPEC §17.1)。
	enumParity = newEnum(map[byte]string{
		0: "none", 1: "even", 2: "odd", 3: "mark", 4: "space",
	})
	// data_bits 反序:0/1/2/3 = 8/7/6/5 bit。
	enumDataBits = newEnum(map[byte]string{
		0: "8", 1: "7", 2: "6", 3: "5",
	})
	enumDHCP     = newEnum(map[byte]string{0: "static", 1: "dhcp"})
	enumFlow     = newEnum(map[byte]string{0: "none", 1: "cts-rts"})
	enumDestMode = newEnum(map[byte]string{0: "static", 1: "dynamic"})
	enumAppProto = newEnum(map[byte]string{0: "transparent", 1: "modbus", 2: "realcom"})
)

// fields 是字段注册表,切片顺序即 info 默认展示顺序。权威偏移见 SPEC §3。
var fields = []Field{
	{Name: "local_ip", Offset: 0, Size: 4, Kind: KindIPv4, Group: "network", Desc: "本地 IP 地址"},
	{Name: "net_mask", Offset: 4, Size: 4, Kind: KindIPv4, Group: "network", Desc: "子网掩码"},
	{Name: "gateway", Offset: 8, Size: 4, Kind: KindIPv4, Group: "network", Desc: "网关"},
	{Name: "dest_ip", Offset: 12, Size: 4, Kind: KindIPv4, Group: "network", Desc: "目的 IP(有 DNS 功能产品此字段无效)"},
	{Name: "local_port", Offset: 16, Size: 2, Kind: KindU16BE, Group: "network", Desc: "本地端口"},
	{Name: "dest_port", Offset: 18, Size: 2, Kind: KindU16BE, Group: "network", Desc: "目的端口"},
	{Name: "work_mode", Offset: 20, Size: 1, Kind: KindEnum, Enum: enumWorkMode, Group: "network", Desc: "工作模式"},
	{Name: "key", Offset: 21, Size: 10, Kind: KindRaw, Group: "advanced", Desc: "密码/key(hex;勿随意清零)"},
	{Name: "devid", Offset: 31, Size: 6, Kind: KindMAC, ReadOnly: true, Group: "identity", Desc: "设备唯一标识(MAC)"},
	{Name: "baud", Offset: 37, Size: 1, Kind: KindEnum, Enum: enumBaud, Group: "serial", Desc: "波特率"},
	{Name: "dev_name", Offset: 38, Size: 10, Kind: KindCString, Group: "identity", Desc: "设备名称"},
	{Name: "parity", Offset: 48, Size: 1, Kind: KindEnum, Enum: enumParity, Group: "serial", Desc: "校验位(even/odd 待真机验证)"},
	{Name: "gap_time", Offset: 49, Size: 1, Kind: KindU8, Group: "serial", Desc: "串口打包间隔时间"},
	{Name: "packing_len", Offset: 50, Size: 2, Kind: KindU16BE, Group: "serial", Desc: "打包长度(1..1400)"},
	{Name: "f_end_en", Offset: 52, Size: 1, Kind: KindU8, Group: "deprecated", Desc: "帧尾有效位(V1.472 起失效)"},
	{Name: "f_end_byte", Offset: 53, Size: 1, Kind: KindU8, Group: "deprecated", Desc: "帧尾字符(失效)"},
	{Name: "f_start_en", Offset: 54, Size: 1, Kind: KindU8, Group: "deprecated", Desc: "帧首有效位(失效)"},
	{Name: "f_start_byte", Offset: 55, Size: 1, Kind: KindU8, Group: "deprecated", Desc: "帧首字符(失效)"},
	{Name: "dhcp_en", Offset: 56, Size: 1, Kind: KindEnum, Enum: enumDHCP, Group: "network", Desc: "IP 模式(静态/DHCP)"},
	{Name: "flow_ctrl", Offset: 57, Size: 1, Kind: KindEnum, Enum: enumFlow, Group: "serial", Desc: "流控方式"},
	{Name: "dest_mode", Offset: 58, Size: 1, Kind: KindEnum, Enum: enumDestMode, Group: "behavior", Desc: "目的模式(静态/动态)"},
	{Name: "data_bits", Offset: 59, Size: 1, Kind: KindEnum, Enum: enumDataBits, Group: "serial", Desc: "数据位"},
	{Name: "app_proto", Offset: 60, Size: 1, Kind: KindEnum, Enum: enumAppProto, Group: "behavior", Desc: "转化协议"},
	{Name: "status", Offset: 61, Size: 1, Kind: KindBitByte, ReadOnly: true, Group: "identity", Desc: "状态(只读)"},
	{Name: "dns_server_ip", Offset: 62, Size: 4, Kind: KindIPv4, Group: "network", Desc: "DNS 服务器 IP"},
	{Name: "dest_string", Offset: 66, Size: 30, Kind: KindCString, Group: "network", Desc: "目的地址串(域名或 IP)"},
	// 96=recon、97=keep_alive:双文档样例标注实锤,勿按 C 结构体顺序改回(SPEC §9)。
	{Name: "recon_time", Offset: 96, Size: 1, Kind: KindU8, Group: "behavior", Desc: "断线重连时间(秒,0..255)"},
	{Name: "keep_alive", Offset: 97, Size: 1, Kind: KindU8, Group: "behavior", Desc: "保活定时时间(秒,0..255)"},
	{Name: "web_port", Offset: 98, Size: 2, Kind: KindU16BE, Group: "network", Desc: "网页访问端口"},
	{Name: "udpf_pos", Offset: 100, Size: 1, Kind: KindU8, Group: "deprecated", Desc: "UDP 滤波(设 0)"},
	{Name: "udpf_code", Offset: 101, Size: 1, Kind: KindU8, Group: "deprecated", Desc: "UDP 滤波(设 0)"},
	{Name: "udpf_mask", Offset: 102, Size: 1, Kind: KindU8, Group: "deprecated", Desc: "UDP 滤波(设 0)"},
	{Name: "ver", Offset: 103, Size: 1, Kind: KindVersion, ReadOnly: true, Group: "identity", Desc: "固件版本(=383+ver)"},
	{Name: "func_sel", Offset: 104, Size: 1, Kind: KindBitByte, ReadOnly: true, Group: "identity", Desc: "支持功能位(只读)"},
	{Name: "group_ip", Offset: 105, Size: 4, Kind: KindIPv4, Group: "network", Desc: "UDP 组播地址(224.0.0.0~239.255.255.255)"},
	{Name: "io_set", Offset: 109, Size: 1, Kind: KindBitByte, Group: "advanced", Desc: "IO 控制字"},
	{Name: "func_en", Offset: 110, Size: 1, Kind: KindBitByte, Group: "advanced", Desc: "功能使能位"},
	{Name: "sm_param_t", Offset: 111, Size: 1, Kind: KindU8, Group: "advanced", Desc: "中心服务器发参间隔(分钟)"},
	{Name: "func_sel2", Offset: 112, Size: 1, Kind: KindBitByte, ReadOnly: true, Group: "identity", Desc: "高级功能位(只读)"},
	{Name: "reserve", Offset: 113, Size: 2, Kind: KindOpaque, Group: "advanced", Desc: "保留"},
	{Name: "user_param", Offset: 115, Size: 52, Kind: KindOpaque, Group: "advanced", Desc: "用户参数区(注册包/心跳包等)"},
}

// bitFields 是位域容器字节的子位定义(SPEC §3 表 C/D/E)。
var bitFields = []BitField{
	{Name: "status.connected", Parent: "status", Byte: 61, Bit: 0, ReadOnly: true, Desc: "TCP 已连接或处于 UDP 态"},

	{Name: "func_sel.web_download", Parent: "func_sel", Byte: 104, Bit: 0, ReadOnly: true, Desc: "支持网页下载"},
	{Name: "func_sel.dns", Parent: "func_sel", Byte: 104, Bit: 1, ReadOnly: true, Desc: "支持 DNS 域名"},
	{Name: "func_sel.realcom", Parent: "func_sel", Byte: 104, Bit: 2, ReadOnly: true, Desc: "支持 REAL_COM 协议"},
	{Name: "func_sel.modbus_tcp_to_rtu", Parent: "func_sel", Byte: 104, Bit: 3, ReadOnly: true, Desc: "支持 Modbus TCP 转 RTU"},
	{Name: "func_sel.serial_param_modify", Parent: "func_sel", Byte: 104, Bit: 4, ReadOnly: true, Desc: "支持串口修改参数"},
	{Name: "func_sel.dhcp", Parent: "func_sel", Byte: 104, Bit: 5, ReadOnly: true, Desc: "支持自动获取 IP(DHCP)"},
	{Name: "func_sel.storage_ex", Parent: "func_sel", Byte: 104, Bit: 6, ReadOnly: true, Desc: "支持存储扩展 EX"},
	{Name: "func_sel.multi_tcp", Parent: "func_sel", Byte: 104, Bit: 7, ReadOnly: true, Desc: "支持多 TCP 连接"},

	{Name: "func_en.data_restart", Parent: "func_en", Byte: 110, Bit: 0, Desc: "数据重启功能"},
	{Name: "func_en.report_to_server", Parent: "func_en", Byte: 110, Bit: 1, Desc: "向中心服务器发送模块参数"},
	{Name: "func_en.need_password", Parent: "func_en", Byte: 110, Bit: 2, Desc: "修改参数需密码"},
	{Name: "func_en.udp_recv_broadcast", Parent: "func_en", Byte: 110, Bit: 3, Desc: "UDP 进制接收广播包"},

	{Name: "func_sel2.io_config", Parent: "func_sel2", Byte: 112, Bit: 0, ReadOnly: true, Desc: "支持 IO 配置"},
	{Name: "func_sel2.udp_multicast", Parent: "func_sel2", Byte: 112, Bit: 1, ReadOnly: true, Desc: "支持 UDP 组播"},
	{Name: "func_sel2.multi_target_ip", Parent: "func_sel2", Byte: 112, Bit: 2, ReadOnly: true, Desc: "支持多目标 IP"},
}

var (
	fieldIndex    = map[string]Field{}
	bitFieldIndex = map[string]BitField{}
)

func init() {
	for _, f := range fields {
		fieldIndex[f.Name] = f
	}
	for _, b := range bitFields {
		bitFieldIndex[b.Name] = b
	}
}

// FieldByName 按机器名查字段。
func FieldByName(name string) (Field, bool) {
	f, ok := fieldIndex[name]
	return f, ok
}

// Fields 返回字段注册表的副本(按展示顺序)。
func Fields() []Field {
	out := make([]Field, len(fields))
	copy(out, fields)
	return out
}

// BitFieldByName 按机器名查位域子字段。
func BitFieldByName(name string) (BitField, bool) {
	b, ok := bitFieldIndex[name]
	return b, ok
}

// BitFields 返回位域子字段的副本。
func BitFields() []BitField {
	out := make([]BitField, len(bitFields))
	copy(out, bitFields)
	return out
}

// FieldOptions 返回枚举字段的可选值名(按取值排序);非枚举字段返回 nil。
func FieldOptions(name string) []string {
	f, ok := fieldIndex[name]
	if !ok || f.Kind != KindEnum || f.Enum == nil {
		return nil
	}
	return f.Enum.options()
}
