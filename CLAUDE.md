# CLAUDE.md — zlan 项目须知

ZLAN(上海卓岚 / zlmcu)串口服务器 / 联网模块管理 CLI。Go,双管理通道(UDP 1092 + 串口命令模式)。
完整协议规格见 `SPEC.md`;本文件只记**会让 agent 踩坑、不可从代码常识推导**的点。

## 架构(改哪层动哪层)
- `internal/protocol`:参数 codec(`Param`=167 字节原始数组 + 字段注册表)+ 两种帧 + 段写模型。纯 stdlib、零 I/O。
- `internal/transport`:UDP(discover/单播/monitor)+ 串口;统一 `Conn` 接口。
- `internal/device`:编排(target 解析、读-改-写校验、批量 apply)。
- `internal/cli`:cobra 命令薄层。
- `main.go`:唯一的 `os.Exit`。

改协议偏移/编码只动 `protocol`;`get/set/info/--json` 全部由字段注册表(`fields.go`)派生,**不要写 per-field 分支**。

## 协议铁坑(动 protocol 前必读)
1. **offset 96 = recon_time、97 = keep_alive**(别反)。C 结构体把这俩声明顺序写反了;图2、两份文档样例标注、默认值(重连 12 / 保活 60)三方一致。**勿按 C 结构体改回**——改了 `TestFieldSetGolden` 会红。证据见 SPEC §9。
2. **parity 1/2(even/odd)两份文档矛盾**:UDP 文档 1=even,串口文档 1=odd。当前取 UDP 文档(1=even),**待真机验证**,别当定论。
3. **key@21 是密码,不是填充**。图2 写 "Pad 应全 0",但 C 结构体名 `param_key`,样例里是 ASCII 密码。读-改-写**勿清零**(整块模型天然保留)。
4. **devid@31 = MAC**,设备唯一标识。写入区间若覆盖它必须与设备匹配,否则设备**丢弃全部写入**。串口部分写避开此区即免匹配。
5. **协议无校验和**。别算 CRC。写正确性靠"写后读回校验"(`device.Set`)。
6. **大端 + 1 字节对齐**。codec 按 offset 手动编解码,不靠 Go struct 内存布局。
7. **data_bits 反序**:0/1/2/3 = 8/7/6/5 bit。
8. **baud 含非标准 7200**(index 3),必须查表(`enumBaud`),不能用线性公式。
9. **ver = 383 + 字节值**。F_start/F_end(52–55)自 V1.472 失效。
10. UDP 改参(0x02)必重启必存;串口 0x03 存不重启 / 0x07 存+重启;UDP 0x03 只改串口参数、不重启不存。
11. 改 local_ip/net_mask/gateway/dhcp_en/dns_server_ip → 设备**必重启、可能换网段**,断连是预期。`device.Set` 对这些字段跳过即时读回校验,`cli` 给失联警告。
12. 单播必须用 discover 拿到的 `*net.UDPAddr`(外网设备在 NAT 后,凭 IP 重构 `:1092` 回不去)。

## 待真机验证(SPEC §17,当前均为推定)
parity even/odd 顺序、串口单帧读写上限(现保守分段 64B)、写后有无 ACK、96/97 终验、改参密码编码、设备应答端口。
无真机时 golden 向量保证与官方文档字节一致;上真机后按 SPEC §17 逐条核实并回写本文件 + SPEC。

## 超出本期范围
IO 控制(UDP 0x05/0x06)、中心服务器上报(0x09/0x0a)依赖未提供的外部 PDF,未实现。

## 开发约定
- `go build ./... && go vet ./... && go test ./...` 必须全绿;改文件后 `gofmt -w .`。
- 每个逻辑单元一个 commit,精准 `git add`(禁 `git add -A`)。
- 二进制协议改动必须配 golden 测试(用文档样例字节,不要只做往返自洽)。
