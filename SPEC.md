# zlan CLI 施工图(SPEC）

ZLAN（上海卓岚 / zlmcu）串口服务器 / 联网模块管理命令行工具。
本文件是写代码前的权威依据。任何与本文不符的实现以本文为准；本文与设备真机行为不符的，以真机为准并回写本文 + agent 须知(`CLAUDE.md`/`AGENTS.md`)。

数据来源：
- 《卓岚联网产品 UDP 管理端口协议》ZL DUI 20100427.1.0 Rev.4（下称 **UDP 文档**）
- 《串口修改参数及硬件 TCPIP 协议栈》ZL DUI 20090825.3.0 Rev.3（下称 **串口文档**）

两份文档共享同一个 167 字节参数结构（图2 / 编号说明 / `struct SSServerParam` 三处一致），但帧格式、命令码两套各不相同。UDP Rev.4 新增说明了 `user_param@115` 的 52 字节可变 TLV 区，WiFi 参数就在这里。

---

## 0. 设计原则（落地到代码的硬约束）

1. **参数 codec 与帧/传输彻底解耦**。一套 `Param`（167B）+ 一张字段注册表，同时服务 UDP（整块帧）与串口（按 offset 部分读写）。`get/set/info/--json/apply` 全部从字段注册表派生，不写 per-field 分支。
2. **写操作统一建模为段集合 `[]Segment{off,len,data}`**。UDP 恒为单段全集（offset 0、len 167）；串口按改动字段的最小覆盖区间切段。"整块 vs 部分"不是两条代码路径，而是同一模型的两种取值——特殊情况被数据结构吃掉。
3. **大端 + 1 字节对齐**。所有多字节数值字段大端；codec 按 offset 手动编解码，**不依赖 Go struct 内存布局**（避免编译器填充）。
4. **无校验和**。协议没有 CRC/checksum。写正确性靠**写后读回校验**兜底，不是靠校验和。
5. **唯一 `os.Exit` 在 main**。子命令一律 `RunE` 返回 error，main 统一映射退出码。
6. **诚实**：文档自相矛盾或未演示的，标"待真机验证"，代码注释写"推定"不写"确认"。

---

## 1. 传输层

| 项 | 值 |
|---|---|
| 协议 | UDP（网络通道）/ 串口（命令模式通道） |
| UDP 管理端口 | **1092** |
| UDP 帧 | `[0x5A 0x4C][cmd 1B][param 167B]` = **170B** |
| UDP magic | `'Z''L'` = `0x5A 0x4C`，线上固定 `5A 4C`（大端） |
| 串口帧 | `[ed f2 a3 56 ca db 91 84 b0 d7][cmd 1B][pos 1B][len 1B][data...]` |
| 串口 magic | 固定 10 字节 `ED F2 A3 56 CA DB 91 84 B0 D7` |
| 串口读返回 | 直接返回 `len` 字节裸参数数据（**无帧头**） |
| 数值字节序 | 大端（high byte 在左）。例 `c0 a8 01 a9`=192.168.1.169；`10 64`=4196 |
| 校验和 | **无** |

**串口命令字 bit0**：1=写参数，0=读参数。`pos`=参数块内偏移，`len`=本次读/写字节数。

---

## 2. 命令码表

### 2.1 UDP 通道命令（帧第 3 字节）

| 码 | 名称 | 方向 | 语义 | 重启 | 保存 |
|---|---|---|---|---|---|
| `0x00` | 广播查询 | PC→设备(广播) | 查所有设备，参数区可全 0 | 否 | 否 |
| `0x01` | 设备应答 | 设备→PC | 回完整参数；TCP 客户端设备也按保活周期主动发此包到目的端口 | 否 | 否 |
| `0x02` | 参数修改 | PC→设备(单播/广播) | 写参数。**参数区 DevID 必须匹配目标**。也用作重启 | **是** | **是** |
| `0x03` | 设串口参数 | PC→设备 | 只改串口参数，**不重启不保存** | 否 | 否 |
| `0x04` | 一对一查询 | PC→设备(单播) | 同 0x00 但发往指定 IP，设备非广播回 0x01 | 否 | 否 |
| `0x05`~`0x0a` | IO / RTS / 中心服务器 | — | **超范围**（依赖未提供的外部文档） | — | — |

### 2.2 串口通道命令（帧第 11 字节）

| 码 | 语义 | 重启 | 保存 | 本期用途 |
|---|---|---|---|---|
| `0x00` | 读参数 | 否 | 否 | info/get/status |
| `0x01` | 写参数（不存；但 IP/掩码/网关/DHCP/DNS 改后会自动重启并保存） | 视字段 | 否 | tune（临时串口参数） |
| `0x03` | 写参数 + 存 Flash | 视字段 | 是 | set（持久改配置） |
| `0x07` | =0x03 + 一定重启 | 是 | 是 | reboot（`07 1f 01 00`）、set --reboot |
| `0x02` | MDIP 多目标 IP（不关旧 TCP） | 否 | 否 | 暂不实现 |
| `0x04`/`0x06`/`0x08`/`0x09`/`0x0a`/`0x0b`/`0x0c`/`0x0d` | 加密狗 / 网页 / 中心服务器 / 专用 | — | — | **超范围** |

---

## 3. 167 字节参数块（权威偏移表）

偏移 = 参数块内偏移（**不含**帧头）。UDP 完整包中再 +3。三来源（图2 / 编号 / C 结构体）交叉验证；歧义处用样例字节定夺。

| off | size | 字段(machine-name) | C 结构体名 | kind | 取值编码 | 备注 |
|---:|---:|---|---|---|---|---|
| 0 | 4 | `local_ip` | param_local_ip | ipv4 | 大端 IP | 本地 IP |
| 4 | 4 | `net_mask` | param_net_mask | ipv4 | 大端 | 子网掩码 |
| 8 | 4 | `gateway` | param_gate_way | ipv4 | 大端 | 网关 |
| 12 | 4 | `dest_ip` | param_dest_ip | ipv4 | 大端 | **有 DNS 功能产品此字段无效**，改用 dest_string |
| 16 | 2 | `local_port` | param_local_port | u16be | 大端 | 本地端口 |
| 18 | 2 | `dest_port` | param_dest_port | u16be | 大端 | 目的端口 |
| 20 | 1 | `work_mode` | param_work_mode | enum | 0=TCP Server,1=TCP Client,2=UDP,3=UDP 组播 | |
| 21 | 10 | `key` | param_key[10] | rawbytes | 密码/key | **非填充！读-改-写勿清零**（详见坑） |
| 31 | 6 | `devid` | ether_addr[6] | rawbytes(readonly) | MAC，6 字节 | **设备唯一标识=MAC**；改参须匹配；部分写避开此区即免匹配 |
| 37 | 1 | `baud` | baundrate_index | enum | 见表 A（含非标准 7200） | 波特率索引 |
| 38 | 10 | `dev_name` | dev_name[10] | cstring | 以 0 结尾可见串 | 厂家工具可能写 GBK/ANSI 中文名；CLI 读写按 UTF-8→GBK 兼容 |
| 48 | 1 | `parity` | param_parity | enum | 见表 B（**两文档矛盾，待验证**） | |
| 49 | 1 | `gap_time` | ...interval | u8 | 串口打包间隔 | |
| 50 | 2 | `packing_len` | param_max_data_len | u16be | 1..1400 | 打包长度 |
| 52 | 1 | `f_end_en` | param_fram_end_en | u8 | 0/1 | **V1.472 起帧尾功能失效** |
| 53 | 1 | `f_end_byte` | param_frame_end_byte | u8 | | V1.472 起帧尾功能失效 |
| 54 | 1 | `f_start_en` / `rs485_half_gap` | param_fram_start_en | u8 | 0..255 | 旧帧首有效位；Rev.4/DLL 作为 485 半双工等待时间(`PARAM_485_GAP`) |
| 55 | 1 | `f_start_byte` / `rs485_timeout` | param_frame_start_byte | u8 | 0..255 | 旧帧首字符；Rev.4/DLL 作为 485 等待最长时间(`PARAM_485_TIME_OUT`) |
| 56 | 1 | `dhcp_en` | param_ip_mode | enum | 0=静态 IP,1=DHCP | |
| 57 | 1 | `flow_ctrl` | param_flow_control | enum | 0=无,1=CTS/RTS,2=DSR/DTR,3=XON/XOFF | |
| 58 | 1 | `dest_mode` | param_dest_dynamic | enum | 0=静态,1=动态 | |
| 59 | 1 | `data_bits` | param_data_bits | enum | **0=8bit,1=7bit,2=6bit,3=5bit**（反序，查表） | |
| 60 | 1 | `app_proto` | app_protocol | enum | 0=透明,1=Modbus TCP↔RTU,2=RealCom | |
| 61 | 1 | `status` | status | u8(readonly) | bit0=1 已连接或 UDP 态 | 只读 |
| 62 | 4 | `dns_server_ip` | dns_server_ip | ipv4 | 大端 | DNS 服务器 |
| 66 | 30 | `dest_string` | dns_name[30] | cstring | 目的地址串，末尾 0 | 改目的地址改这里 |
| **96** | 1 | `recon_time` | (图2 recon_time) | u8 | 断线重连时间 0..255（秒） | **见 §9 裁决：recon 在前** |
| **97** | 1 | `keep_alive` | (图2 keep_alive) | u8 | 保活定时时间 0..255（秒） | |
| 98 | 2 | `web_port` | web_port | u16be | 大端 | 网页访问端口 |
| 100 | 1 | `udpf_pos` | udp_filter_pos | u8 | 设 0（弃用） | |
| 101 | 1 | `udpf_code` | udp_filter_code | u8 | 设 0 | |
| 102 | 1 | `udpf_mask` | udp_filter_mask | u8 | 设 0 | |
| 103 | 1 | `ver` | ver | u8(readonly) | **版本号 = 383 + ver** | ver=0→1.383，ver=117→1.500 |
| 104 | 1 | `func_sel` | func_sel | bitfield(readonly) | 见表 C | 只读功能位 |
| 105 | 4 | `group_ip` | udp_group_ip | ipv4 | 大端，224.0.0.0~239.255.255.255 | 组播地址 |
| 109 | 1 | `io_set` | io_set | bitfield | IO 控制字（IO 文档） | |
| 110 | 1 | `func_en` | func_en | bitfield | 见表 D | 可写使能位 |
| 111 | 1 | `sm_param_t` | server_mode_param_t | u8 | 中心服务器发参间隔（分钟） | |
| 112 | 1 | `func_sel2` | func_sel2 | bitfield(readonly) | 见表 E | 只读高级功能位 |
| 113 | 1 | `multi_host_wait` | var1[0] / maxwait | u8 | 0=关闭,1=25ms,2=50ms | 485 多主机时间 |
| 114 | 1 | `stop_bits` | var1[1] / OtherBits | stopbits | bit0=0→1 位,bit0=1→2 位 | CLI 写入只改 bit0、保留其它位 |
| 113 | 2 | `reserve` | var1[2] | opaque | 原始保留区 | 纯透传不动；与 `multi_host_wait`/`stop_bits` 同区间 |
| 115 | 52 | `user_param` | var2[52] | tlv/opaque | 可变用户区（WiFi / 多目的 IP / 计数器等） | 见 §3.1；未知 TLV 纯透传 |

合计 115 + 52 = **167** ✓

### 表 A — baud（波特率索引，查表非公式）
`0=1200 1=2400 2=4800 3=7200 4=9600 5=14400 6=19200 7=28800 8=38400 9=57600 10=76800 11=115200 12=230400 13=460800`

### 表 B — parity（**两文档矛盾，待真机验证**）
- UDP 文档：`0=None 1=Even 2=Odd 3=Mark 4=Space`
- 串口文档：`0=None 1=Odd 2=Even 3=Mark 4=Space`
- 0/3/4 一致；**1 与 2（Even/Odd）相反**。暂以 UDP 文档为默认，CLI 用名字（none/even/odd/mark/space）暴露，映射值标"待验证"。

### 表 C — func_sel（只读）
`bit0`网页下载 `bit1`DNS `bit2`REAL_COM `bit3`Modbus TCP转RTU `bit4`串口改参 `bit5`DHCP `bit6`存储扩展EX `bit7`多TCP连接

### 表 D — func_en（可写）
`bit0`数据重启功能 `bit1`向中心服务器发参 `bit2`修改参数需密码 `bit3`UDP 进制接收广播包 `bit4`P2P `bit5`连接上发送 MAC `bit6`ping 检测断网 `bit7`连接上时不清空缓存

### 表 E — func_sel2（只读）
`bit0`IO 配置 `bit1`UDP 组播 `bit2`多目标 IP `bit3`代理服务器 `bit4`SNMP `bit5`P2P

### 3.1 user_param@115 可变 TLV 区（UDP Rev.4）

`user_param` 是参数块 offset 115..166 的 52 字节区域。Rev.4 将它定义为连续 TLV：

`type 1B | len 1B | value lenB | ... | type=0 结束 | 后续补 0`

写入仍走普通 UDP `0x02` 整块写：先 `0x04` 读整块，修改 `user_param` TLV，保持参数总长 167，再写回。不要把 WiFi 当作固定 offset 字段加入主字段表。

已知 TLV 类型：

| type | 名称 | 说明 |
|---:|---|---|
| `0` | 结尾 | 停止解析 |
| `1` | 功能选择 | 文档未展开 |
| `2` | WiFi SSID | 原始 SSID 字节，不带结尾 0 |
| `3` | WiFi 信道/桥接/DHCP | 1 字节位域，见下 |
| `4` | WiFi 密码 | 原始密码字节，不带结尾 0 |
| `5` | WiFi 密码类型 + STA/AP | 1 字节位域，见下 |
| `6` | 多目的 IP | 文档未展开 |
| `8` | 代理服务器参数 | 文档未展开 |
| `9` | 串口发送字节数 | 查询时自动添加，写入时可不填 |
| `10` | 串口接收字节数 | 查询时自动添加，写入时可不填 |
| `255` | 自定义 | 厂家/用户自定义 |

TLV type `3`（信道/桥接/DHCP）长度为 1：

- `bit0..bit3`：WiFi 信道号（文档示例直接用信道 4 编码为低半字节 `4`）
- `bit6=1`：关闭 DHCP Server（注意 1 表示关闭）
- `bit7=1`：开启以太网/WiFi 互通桥接

例：`03 01 44` = 信道 4、桥接关闭、DHCP Server 关闭。

TLV type `5`（密码类型 + STA/AP）长度为 1：

- `bit0..bit5`：密码类型
- `bit6=1`：AP 模式
- `bit7=1`：STA 模式

密码类型取值（这是 Rev.4 TLV 线上编码，注意与 ZLDevManage DLL UI 顺序不同）：

| 值 | 含义 |
|---:|---|
| `0` | 无加密 |
| `1` | WEP64 |
| `2` | TKIP |
| `4` | AES |
| `5` | WEP128 |
| `6` | 自动 |

例：`05 01 86` = STA 模式 + 自动密码；`05 01 40` = AP 模式 + 无加密。

实现要求：

- 解析器必须容忍未知 TLV，写回时尽量保留未知 TLV；
- 重建后必须不超过 52 字节，并以 `type=0` 结束、剩余补 0；
- 查询包中的 type `9`/`10` 计数器是状态类数据，写 WiFi 时可不主动生成；
- WiFi 写命令应走专用 TLV builder，不走普通 `SetField` 固定偏移。

---

## 4. 字段注册表设计

每个字段一条记录：`{name, offset, size, kind, readonly, enc, dec, validate}`。

- **kind**：`ipv4 | u16be | u32be | u8 | enum | cstring | rawbytes | bitfield | opaque | tlv`
- **enc/dec**：枚举/反序字段用查表（baud、parity、data_bits、flow_ctrl），不用线性公式；`stop_bits` 只改 OtherBits bit0，保留其它未知位
- **validate**：枚举走白名单；IP 走格式；`packing_len`∈[1,1400]；`group_ip`∈[224.x,239.x]；端口∈[0,65535]
- **位域子字段**：`func_en`/`io_set`/`func_sel`/`func_sel2` 是容器，其下挂子字段如 `func_en.need_password = {byte:110, bit:2}`。set 位域 = 读回容器字节 → 改位 → 写回；CLI 也接受整字节 `func_en=0x05`
- **cstring**：写时补 0、清掉旧残留；`dev_name` 兼容厂家工具 GBK/ANSI 中文名；**rawbytes（key）**：定长，不足补 0x00 但不主动清零未改部分；**opaque（reserve/未知 user_param TLV）**：只透传，永不主动改；**TLV（user_param）**：只由专用 parser/builder 改 WiFi 等子结构

---

## 5. 寻址决策表

| `--serial` | target 位置参数 | 行为 |
|---|---|---|
| 未设 | IP（如 `192.168.1.200`） | UDP 单播到 `IP:1092`（0x04 查 / 0x02 改） |
| 未设 | devid/MAC（如 `5a:4c:6f:73:cc:d6`） | 先广播 discover 匹配 DevID → 拿到 `*net.UDPAddr` → 单播 |
| 未设 | 省略（discover/monitor） | 广播 / 监听 |
| 已设（如 `/dev/cu.usbserial-x`） | 省略 | 串口直连单台；**传 target 报错**（通道二选一） |

- target 解析返回 `{devid, addr *net.UDPAddr, snapshot []byte}`，**不是裸 IP**。
- **外网/跨 NAT 设备**：改参目的地址不是 `IP:1092`，而是设备 0x01 上报包的**源地址**（由 monitor 捕获并透传），且须在设备发包后 1 分钟内回。
- 删除冗余的 `--host` flag，寻址只有 target 一个入口。

---

## 6. 命令树与语义

```
zlan discover [--timeout] [--target IP --bind IP]   发现设备（UDP 广播 + 小网段单播探测），表格 DEVID/NAME/IP/MODE/BAUD/VER/STATUS
zlan capabilities [target]             读取/对比 func_sel、func_sel2 能力位
zlan info    <target>                     读完整参数，分组展示
zlan get     <target> <field>             读单字段（脚本友好）；get <target> --list-fields 列字段
zlan set     <target> <k=v>...            改配置（持久，会重启），破坏性
zlan set     <target> --profile modbus-tcp-rtu      套用 Modbus TCP→RTU 常用网关配置
zlan tune    <target> baud=.. parity=..   临时串口参数（不存不重启，断电恢复）
zlan wifi get <target> [--show-key]       读取 user_param@115 中的 WiFi TLV 参数
zlan wifi set <target> ssid=.. key=.. mode=sta crypt=auto channel=.. dhcp_server=disabled bridge=disabled
zlan copy    <source> <target> [k=v]...   复制 source 可写配置到 target（默认 dry-run；--confirm 写入）
zlan export  <target> [-o file]           导出离线配置文件（param_hex 为权威原始参数块）
zlan import  <target> -f file [k=v]...    从配置文件导入到 target（默认 dry-run；--confirm 写入）
zlan reboot  <target>                     重启
zlan status  <target>                     连接状态（轻量探针，脚本 watch 用）
zlan reg read <target> <addr> [count]     经数据通道读取 Modbus 寄存器
zlan reg write <target> <addr> <value...> 经数据通道写入 Modbus holding register
zlan reg poll <target> <addr> [count]     建立数据通道长连接，定时循环读取寄存器
zlan reg session <target>                 建立数据通道长连接，交互式读写寄存器
zlan monitor [--listen :1092]             被动监听设备 0x01 周期上报；可对外网设备改参
zlan apply   -f devices.yaml [--dry-run] [--confirm]   批量声明式配置
zlan ports                                列出本机可用串口
zlan version                              版本（亦 --version）
zlan completion [bash|zsh|fish]           shell 补全（cobra 自动）
```

### 命令 → 通道 → 命令码

| CLI | UDP 通道 | 串口通道 | 重启 | 保存 |
|---|---|---|---|---|
| discover | 0x00 广播 + 0x04 小网段单播探测 | —（串口单台直接 info） | 否 | 否 |
| capabilities | 读 func_sel/func_sel2 能力位;无 target 时先 discover 汇总 | 0x00 读 @104/@112 或整块读 | 否 | 否 |
| info/get | 0x04 单播 / 0x00 广播匹配 | 0x00 读 | 否 | 否 |
| set | 0x02（读-改-写整块） | 0x03（写+存；网络字段自动重启） | UDP:是 | 是 |
| tune | 0x03 | 0x01（写不存） | 否 | 否 |
| wifi get | 0x04 后解析 `user_param@115` TLV | 0x00 读整块后解析 TLV | 否 | 否 |
| wifi set | 0x04 读整块→改 `user_param@115` TLV→0x02 整块写 | 0x00 读整块→写 `user_param@115` 段 | UDP:是 | 是 |
| copy | 源/目标读参后按 set 逻辑写 target；保留 target 的 devid/只读字段 | 暂不支持 `--serial`（需同时访问两台） | 依字段 | 是 |
| export/import | export 单台读参生成文件；import 读文件后按 copy 逻辑写 target | import 支持单台串口目标；export 支持串口读参 | 依字段 | 是 |
| reboot | 0x04 读回→改 0x02 回发（§3.5） | 0x07 `07 1f 01 00`（§3.7） | 是 | UDP:是 |
| status | 0x04 读 @61 | 0x00 pos=61 len=1 | 否 | 否 |
| reg read/write | 先管理通道读参数,再连接数据 TCP 通道跑 Modbus TCP 或 RTU-over-TCP | 暂不支持 | 否 | 依 Modbus 写入 |
| reg poll/session | 同上,但保持数据 TCP 长连接;断线时按需重连 | 暂不支持 | 否 | 依 Modbus 写入 |
| monitor | 监听端口收 0x01；改参把 0x01→0x02 回发 | — | — | — |
| apply | 每台走 set 逻辑（按 devid 匹配） | （一般 UDP） | 依字段 | 是 |

### 全局 flag
`--serial <path>` `--baud <n>`（切串口通道；--baud 须匹配模块当前波特率，默认 115200）
`--json` `-q/--quiet` `-v/--verbose` `--no-color` `-y/--yes` `--confirm` `--timeout` `--config <path>`
（`--quiet` 压 `--verbose`；密码不走 flag，见 §11）

`discover` 额外支持 `--target <broadcast-ip>`（可重复/逗号分隔）、`--bind <local-ip>`、
`--bind-port <port>`。默认按所有 up 且支持广播的 IPv4 网段发定向广播并追加
`255.255.255.255` 兜底；对主机数不超过 512 的本地网段补充 `0x04` 单播探测,
用于发现不响应广播但响应单播查询的设备。显式 flag 用于复现现场多网卡/跨网段扫描。

`set --profile modbus-tcp-rtu` 展开为:
`work_mode=tcp-server local_port=502 app_proto=modbus baud=9600 parity=none data_bits=8`。
用户同一命令里显式给出的 `field=value` 覆盖 profile 默认值。

---

## 7. 写流程与读回校验

1. **读**：UDP 读回整块 167B（0x04）/ 串口读所需区间（0x00）→ 解码为 `Param`（`snapshot`）。
2. **改**：在 `snapshot` 副本上按 `k=v` 改字段（位域读-改-位；cstring 补 0；opaque 不动；只读字段保持原值回写——§3.5 证实设备接受只读区原值回写）。
3. **算段**：UDP→单段全集；串口→改动字段最小覆盖区间（合并相邻字段；避开 devid 区则免匹配）。
4. **写**：UDP 0x02 / 串口 0x03（或 0x07）。
5. **读回校验**：重新读改动字段，比对是否生效。
   - **例外**：改网络类字段（`local_ip/net_mask/gateway/dhcp_en/dns_server_ip`）→ 设备重启，**不做即时读回校验**，改为"写入完成 + 尽力可达性确认"（见 §10）。

---

## 8. 批量 apply（声明式）

```yaml
# devices.yaml —— 按 devid(MAC) 匹配，IP 改完即变，不能当主键
- match: { devid: "5a:4c:6f:73:cc:d6" }
  set:   { local_ip: 10.0.0.11, net_mask: 255.255.255.0, gateway: 10.0.0.1 }
- match: { devid: "5a:4c:6f:73:cc:d7" }
  set:   { local_ip: 10.0.0.12 }
```

- **默认 `--dry-run`**：先 discover，逐台打印 before→after diff，不写。
- 真正执行需 `--confirm`（批量/网络变更，`-y` 不够；非 TTY 必须显式 `--confirm`）。
- **串行**执行，每台独立结果（成功/失败/已写入未确认），末尾汇总。默认失败不中断（`--fail-fast` 可选）。
- "对多台一次改 IP"=逐台施加不同配置（provisioning），不是广播；CLI 不提供广播改 IP。

---

## 9. 关键裁决：offset 96/97（recon vs keep_alive）

**裁定：offset 96 = `recon_time`（断线重连），offset 97 = `keep_alive`（保活）。**

证据（三重，双文档）：
1. UDP 文档图2：`recon_time`@96、`keep_alive`@97；
2. UDP 文档 §3.1 样例该处字节 `0c 3c`；
3. **串口文档 §3.6 带逐字段中文标注的写命令**：`…0c(重连时间) 3c(保活定时时间) 00 50(网页端口)…`，且 `0x0c=12`/`0x3c=60` 精确等于设备默认"断线重连 12 秒 / 保活 60 秒"（§3.6 图6）。

`struct SSServerParam` 把 `keep_alive_time` 声明在 `reconnect_time` 之前——**与图2、两份样例标注矛盾，不采信**（人写结构体成员顺序写反极常见，样例字节不会撒谎）。

置信度：高（双独立文档 + 标注 + 默认值三方吻合）。真机 `set recon=15,keep_alive=90` 读回为最终偏执确认（非阻塞）。

---

## 10. 安全护栏

- **破坏性操作分级**：单台 `set/reboot/tune` → `-y` 可跳过 yes/no 确认；**批量 `apply`、`copy/import` 写入或改网络类字段** → 需 `--confirm`，非 TTY 无 `--confirm` 直接报错退出（绝不阻塞）。
- **确认提示明说后果**：`set: 将保存配置并重启设备(断开 TCP 连接)。继续?`，不是干巴巴 "Are you sure?"。
- **改 IP 不误报**：写网络字段后先报"写入完成、设备重启中"；可达性确认为**尽力而为**，跨网段失败提示"设备可能已切到新网段，请在对应网段 `zlan discover`"，**退出码 0 + 警告**，绝不用超时码。
- `dhcp_en=1` 时 set `local_ip` 给出无意义警告（DHCP 会覆盖）。

---

## 11. 密码处理（func_en bit2「改参需密码」）

优先级：`ZLAN_PASSWORD` 环境变量 → 交互 prompt（TTY，`x/term.ReadPassword` 不回显）→ `--password-file <path>` / `--password-stdin` → 配置文件（权限 0600，文档警告）。
**绝不提供 `--password` 明文 flag**（会泄到 `ps`/shell history）。非 TTY 且无任何来源 → 报错退出（码 6 或 2）。
（密码具体如何编码进包对应 key@21 字段，细节文档缺失，待真机验证。）

---

## 12. 配置优先级

`--flag` > 环境变量 `ZLAN_*` > 项目配置 `./.zlan.yaml`（可选） > 用户配置 `os.UserConfigDir()/zlan/config.yaml` > 内置默认。
`--config <path>` 显式指定时跳过搜索。配置可存：默认 timeout、默认通道、已知设备别名。

---

## 13. 退出码

| 码 | 含义 |
|---|---|
| 0 | 成功（含"已写入但跨网段无法确认"，配 stderr 警告） |
| 1 | 一般错误 |
| 2 | 用法错误（flag 解析失败归此） |
| 3 | 设备未找到 / 无应答匹配 |
| 4 | 网络超时（无响应） |
| 5 | 协议解析 / 帧校验失败 |
| 6 | 鉴权 / 密码失败 |
| 7 | 批量部分失败 |
| 130 | Ctrl-C 取消（128+SIGINT） |

---

## 14. 输出约定（clig.dev）

- **数据走 stdout，进度/日志/错误走 stderr**（`zlan discover --json \| jq` 不被污染）。用 `cmd.OutOrStdout()`/`ErrOrStderr()`，不裸 `fmt.Println`。
- `--json`：stdout 只有合法 JSON；JSON 模式下错误以结构化 `{"error":{...}}` 走 **stderr**。`--json` 输出含 `schema_version` 字段。
- 颜色：`NO_COLOR` 非空 / `TERM=dumb` / 非 TTY / `--no-color` 任一 → 关色（启动判一次）。
- 非 TTY 绝不交互阻塞；需确认而无 `-y/--confirm` → 报错教用户加 flag。
- `--version` + `version` 子命令，版本号用 `-ldflags -X` 注入（含 git commit / build date）。

---

## 15. 架构与文件

```
zlan/
  main.go                       唯一 os.Exit；signal.NotifyContext(Ctrl-C)
  internal/
    protocol/
      param.go                  Param(167B) + 命令码常量
      fields.go                 字段注册表 + 位域子表 + 查表枚举
      codec.go                  Param<->[]byte（大端、按 offset、查表）
      frame_udp.go              5A4C 帧编解码
      frame_serial.go           ED..D7 帧编解码（pos/len 部分读写）
      segment.go                []Segment 写计划（整块=全集特例）
      codec_test.go             golden hex（§16）+ 往返 + 坏输入不 panic
    transport/
      transport.go              interface{ ReadSeg/WriteSeg/Discover/Monitor }
      udp.go                    定向广播逐网卡 + 总 deadline 收齐 + DevID 去重；单播 UDPAddr+重试；monitor 原路回包(UDP 无 TIME_WAIT、discover 用临时端口,故不需 REUSEADDR)
      udp_test.go               127.0.0.1:0 fake echo 设备；ip|^mask 纯函数测
      serial.go                 go.bug.st/serial，分段读写（保守上限）
    device/
      service.go                Discover/Info/Get/Set/Tune/Reboot/Status/Monitor 编排 + 写后读回校验
      target.go                 IP / devid 解析 → {devid,addr,snapshot}
      apply.go                  清单解析 + dry-run diff + 串行执行 + 汇总
    cli/
      root.go discover.go info.go get.go set.go tune.go reboot.go
      status.go monitor.go apply.go ports.go version.go
      confirm.go                非 TTY 不阻塞确认
      errors.go                 ExitError + 退出码映射
      output.go                 table(tabwriter)/json/color helper
    config/config.go            os.UserConfigDir + 优先级链
  CLAUDE.md  AGENTS.md(软链)  README.md  SPEC.md
```

依赖：`spf13/cobra` + `go.bug.st/serial` + `golang.org/x/term` + `golang.org/x/text` + `gopkg.in/yaml.v3`，其余 stdlib。

---

## 16. golden 测试向量（来自文档精确样例）

字段级（串口命令，最可信，无 OCR 漂移）：

| 来源 | 命令字节 | 含义 |
|---|---|---|
| 串口 §3.13 | `01 12 02 04 01` | 写 `dest_port`@18 = `0x0401`=1025（大端） |
| 串口 §3.4 | `01 3e 04 08 08 08 08` | 写 `dns_server_ip`@62 = 8.8.8.8 |
| 串口 §3.12 | `01 42 1e 31 39 32 2e 31 36 38 2e 31 2e 31 38 38 00 …` | 写 `dest_string`@66 = "192.168.1.188\0" |
| 串口 §3.9 | `00 26 0a` | 读 `dev_name`@38 共 10B |
| 串口 §3.11 | `00 1f 06` | 读 `devid`@31 共 6B |
| 串口 §3.14 | `00 67 01` | 读 `ver`@103 共 1B（值+383） |
| 串口 §3.1 | `00 3d 01` | 读 `status`@61 共 1B |

整块逐字段（串口 §3.6「一次设置」写 0x68=104B from 0，覆盖大部分字段并实锤 96/97）：
`local_ip`@0=192.168.1.200 / `dest_port`@18=80 / `work_mode`@20=1(TCP Client) / `devid`@31=`5a 4d 01 02 03 04` / `baud`@37=`0b`(115200) / `packing_len`@50=`05 14`(1300) / `dest_string`@66="www.baidu.com" / **`recon_time`@96=`0c`(12) / `keep_alive`@97=`3c`(60)** / `web_port`@98=80 / `ver`@103=`12`。

UDP 应答字段锚点（UDP §3.1，尾部 `…8a b6 e6` 对齐到 @100，确认 0..103 无漂移）：
`local_ip`@0=192.168.1.169 / `local_port`@16=4196 / `devid`@31=`5a 4c 6f 73 cc d6` / `baud`@37=`04`(9600) / `packing_len`@50=`05 14`(1300) / `recon`@96/`keep`@97=`0c`/`3c`。

> 注：UDP §3.1、§3.2 的**整包** hex 在 PDF 里有 1~2 字节转写漂移（DestString 被敲到 64/65），**不可逐字节当 golden**；以上只取经尾部锚点验证过的字段。

---

## 17. 待真机验证清单（诊断纪律：以下为推定，非确认）

1. **parity 编码 1/2**：UDP 文档 Even/Odd 与串口文档/ZLDevManage demo UI Odd/Even 相反（表 B）。一次 `set parity=odd` 读回即可定。
2. **串口单帧读/写上限**:文档证实可一次写 104B(§3.6),167B 整块未演示 → 实现用分段 + 保守上限规避。
3. **写后 ACK 行为**:0x02/0x03/0x07 写完是否回 ACK,还是无响应直接重启 → 影响"写入已确认"判定;当前靠读回校验。
4. **96/97 顺序**：双文档标注已确认（§9），真机 `set recon=15,keep_alive=90` 读回为最终确认。
5. **改参密码编码**:func_en bit2 触发条件 + 密码如何编码进 key@21 → 文档缺失。
6. **设备应答端口**:文档(UDP 表1 0x01)称"PC 在发送广播的端口收到应答" → discover 用临时源端口,与 monitor(:1092)不冲突;真机确认。
7. **WiFi TLV 实机行为**:Rev.4 已给线上编码；仍需真机确认 7110M 当前固件是否完全按该 TLV 写入生效、SDK 枚举是否只是 DLL 层转换。

---

## 18. 项目坑清单(写入 agent 须知)

- 96/97 = recon/keep_alive(§9 证据链),**勿按 C 结构体改回**。
- parity 1/2 两文档矛盾,代码标"待验证"。
- `key`@21 是密码非填充,读-改-写勿清零。
- `devid`@31 = MAC,改参须匹配;串口部分写避开此区即免匹配。
- 协议无校验和,写正确性靠读回校验。
- UDP 0x02 必重启必存;0x03 不重启不存。串口 0x03 存不重启 / 0x07 存+重启。
- 改 IP/掩码/网关/DHCP/DNS → 设备必重启,断连是预期不是错误。
- 大端 + 1 字节对齐;codec 按 offset 手动,不靠 struct 内存布局。
- `data_bits` 反序:0/1/2/3 = 8/7/6/5 bit。
- `ver` 换算 = 383 + ver;F_end(52-53)帧尾功能 V1.472 后失效;F_start(54-55)在 Rev.4/DLL 中复用为 485 参数。
- 单播必须用 discover 拿到的 `*net.UDPAddr`,外网设备别 Dial 到 IP:1092。
- baud 表含非标准 7200(index 3),用查表别用公式。
- `user_param@115` 是 Rev.4 TLV 区；WiFi 是 TLV 子结构，**不是固定 offset 字段**。未知 TLV 必须保留，不要清空 52 字节尾区。
