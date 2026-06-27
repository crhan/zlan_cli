# zlan

ZLAN(上海卓岚)串口服务器 / 联网模块管理命令行工具。
通过 **UDP 管理端口(1092)** 或 **串口命令模式** 发现、查看、配置、重启卓岚联网模块。

遵循 [Command Line Interface Guidelines](https://clig.dev):人读表格 + 机器读 `--json`,数据走 stdout、进度/错误走 stderr,破坏性操作确认,完善的退出码与补全。

## 安装

```sh
go build -o zlan .          # 本地构建
# 或 go install(若已发布到模块仓库)
```

发布版从 GitHub Release 下载对应平台压缩包:

- Linux: `zlan_<version>_linux_amd64.tar.gz` / `zlan_<version>_linux_arm64.tar.gz`
- macOS: `zlan_<version>_darwin_amd64.tar.gz` / `zlan_<version>_darwin_arm64.tar.gz`

Windows 暂不发布二进制。校验文件为 `checksums.txt`。

## 快速开始

```sh
zlan discover                              # 发现局域网内所有设备
zlan discover --target 192.168.1.255 --bind 192.168.1.10
zlan capabilities                          # 对比当前设备能力位
zlan info 192.168.1.200                    # 查看一台设备的完整参数
zlan get 192.168.1.200 local_ip            # 读取单个字段(脚本友好)
zlan set 192.168.1.200 dest_port=4196      # 修改配置(会保存并重启设备)
zlan set 192.168.1.200 --profile modbus-tcp-rtu
zlan reg read 192.168.1.200 0x0001 2 --unit 11
zlan reg write 192.168.1.200 0x0002 11 --unit 1
zlan mqtt publish 192.168.1.200 0x0000 --unit 13 --broker localhost:1883 --topic zlan/water/rsds19y/state --field temperature --scale 0.1
zlan copy 192.168.1.200 192.168.1.201      # 复制配置(dry-run 预览)
zlan export 192.168.1.200 -o backup.yaml   # 导出离线配置文件
zlan reboot 192.168.1.200                  # 重启
zlan info --serial /dev/cu.usbserial-1410  # 串口直连(IP 不通时救砖/首次配置)
```

## 命令

| 命令 | 说明 |
|---|---|
| `discover` | 发现局域网设备(别名 `scan`/`ls`) |
| `capabilities [target]` | 查看/对比设备能力位 |
| `info <target>` | 读取并分组展示完整参数 |
| `get <target> <field>` | 读取单字段;`get --list-fields` 列出所有字段 |
| `set <target> <k=v>...` | 修改配置(保存并重启设备) |
| `tune <target> <k=v>...` | 临时设串口参数(不保存、不重启,断电恢复) |
| `copy <source> <target> [k=v]...` | 复制一台设备配置到另一台(默认 dry-run;别名 `clone`/`cp`) |
| `export <target> -o <file>` | 导出可离线携带的配置文件 |
| `import <target> -f <file> [k=v]...` | 从配置文件导入到设备(默认 dry-run) |
| `reboot <target>` | 重启设备 |
| `status <target>` | TCP 连接状态(轻量探针,适合 `watch`) |
| `reg read/write/poll/session <target> ...` | 经 ZLAN 数据通道读写/轮询 Modbus 寄存器 |
| `mqtt publish/bridge <target> ...` | 读取 Modbus 寄存器并发布到 MQTT/HA Discovery |
| `monitor` | 被动监听设备周期上报(0x01) |
| `apply -f <yaml>` | 按清单批量配置(按 DevID 匹配) |
| `ports` | 列出本机可用串口 |

`<target>` 为设备 IP 或 DevID(MAC);用 `--serial <路径>` 时省略。

## 现场常用 profile

`set --profile modbus-tcp-rtu` 会套用 ZLAN7110M 作为 Modbus TCP → RTU 网关的常用配置:

| 字段 | 值 |
|---|---|
| `work_mode` | `tcp-server` |
| `local_port` | `502` |
| `app_proto` | `modbus` |
| `baud` / `parity` / `data_bits` | `9600` / `none` / `8` |

显式传入的 `field=value` 会覆盖 profile 中同名字段。

## 寄存器读写

`reg` 会先读取 ZLAN 当前参数,再根据数据通道状态自动选择访问方式:

- `app_proto=modbus`:连接 `local_ip:local_port`,发送 Modbus TCP。
- `app_proto=transparent`:连接 `local_ip:local_port`,发送带 CRC 的 Modbus RTU 帧。

```sh
zlan reg read 192.168.1.200 0x0001 2 --unit 11
zlan reg read 192.168.1.200 0x0001 --kind input --unit 11 --json
zlan reg write 192.168.1.200 0x0002 0x000b --unit 1
zlan reg poll 192.168.1.200 0x0001 2 --unit 11 --interval 1s
zlan reg session 192.168.1.200 --unit 11
```

`reg poll` 建立一条数据通道长连接,按 `--interval` 反复读取同一组寄存器;`--times N`
可限制采样次数,默认一直运行到 Ctrl-C。`--json` 模式每次采样输出一行 JSON。

当前只主动连接 `work_mode=tcp-server` 的设备。`tcp-client` / `udp` 模式没有本机可直接打开的数据
TCP 监听,需先调整配置,或在对应服务器侧操作。若设备参数不可信,可用 `--mode modbus-tcp|rtu-over-tcp`
和 `--data-host` / `--data-port` 手动覆盖。

## MQTT 上送

`mqtt` 会复用 `reg` 的数据通道解析逻辑,先读取 Modbus 寄存器,再将读数封成 JSON 发布到
MQTT broker。它不是写入设备内部 JSON/MQTT 采集规则,而是由 CLI 作为手动验证或外部调度的桥接进程,
适合先把 RS485 读数接入 Home Assistant MQTT。

```sh
# 一次性发布温度,raw=250 会按 scale=0.1 转为 temperature=25.0
zlan mqtt publish 192.168.15.47 0x0000 \
  --unit 13 --broker localhost:1883 \
  --username ruohan.chen --password-env MQTT_PASSWORD \
  --topic zlan/water/rsds19y/state \
  --field temperature --scale 0.1 --retain

# 持续桥接,每 2 秒读一次并发布;--times 0 表示一直运行
zlan mqtt bridge 192.168.15.47 0x0000 \
  --unit 13 --interval 2s --topic zlan/water/rsds19y/state \
  --field temperature --scale 0.1 --retain

# 同时发布 Home Assistant MQTT Discovery 配置(retained)
zlan mqtt publish 192.168.15.47 0x0000 \
  --unit 13 --ha-discovery --name "RSDS19Y Temperature" \
  --unique-id zlan_rsds19y_temperature --device-class temperature \
  --unit-of-measurement °C --field temperature --scale 0.1
```

MQTT 只实现 v3.1.1 QoS0 publish,支持用户名/密码认证。密码可用 `--password-env` 或
`--password-file` 提供,避免在进程参数里明文暴露。

## 复制配置

`copy` 会从 source 复制可写配置字段到 target,保留 target 自己的 `devid`、状态、固件版本和能力位。
默认只预览 diff,加 `--confirm` 才实际写入并重启 target。若源/目标不方便同时在线,用 `export` / `import` 分两步完成。

```sh
zlan copy 192.168.1.200 192.168.1.201
zlan copy 192.168.1.200 192.168.1.201 local_ip=192.168.1.201 --confirm
zlan copy 192.168.1.200 192.168.1.201 --exclude local_ip --confirm

zlan export 192.168.1.200 -o zlan-200.yaml
zlan import 192.168.1.201 -f zlan-200.yaml
zlan import 192.168.1.201 -f zlan-200.yaml local_ip=192.168.1.201 --confirm
zlan import --serial /dev/cu.usbserial-1410 -f backup.yaml --confirm
```

复制 `local_ip` 可能造成两台设备 IP 冲突;替换设备时可显式覆盖目标 `local_ip`,批量铺同类配置时可 `--exclude local_ip`。
导出文件中 `param_hex` 是 import 使用的权威原始参数块;`fields` 只是便于人工查看的快照。

## 两条通道

- **UDP(默认)**:设备已联网时用,`zlan <命令> <IP|DevID>`。
- **串口**:IP 配错连不上时救砖、首次本地配置,加 `--serial <路径> --baud <波特率>`(波特率须匹配设备当前值;`zlan ports` 查路径,macOS 选 `/dev/cu.*`)。

两条通道共享同一套 167 字节参数,字段名、取值完全一致。

`discover` 默认按所有本机 IPv4 网段发定向广播,并对较小本地网段补充 UDP 单播探测;
遇到多网卡/跨网段路由时,可用 `--target` 指定广播地址,用 `--bind` 指定本机源地址。
部分真机固件只稳定响应广播查询,也有设备只响应 `0x04` 单播查询。
运行时会逐个出口打印"从哪个网卡/源地址发出、覆盖了哪些广播目标",一眼看清扫描范围:

```
  出口 [ens19] 192.168.14.15:33549 → 广播 192.168.15.255, 255.255.255.255(单播探测 509)
  出口 [docker0] 172.17.0.1:33121 → 广播 172.17.255.255, 255.255.255.255
```

**找不到出厂默认 IP 的设备?** 设备默认 IP 多为 `192.168.1.200`。若本机没有该网段,
自动扫描既不发该网段定向广播也不做单播探测,只剩 `255.255.255.255` 兜底,设备能否
跨子网应答取决于固件,不保证。**最稳的办法是先在网卡上加一个该网段的辅助地址**,例如:

```
sudo ip addr add 192.168.1.9/24 dev eth0    # Linux,临时;掉电/重启失效
```

之后 `zlan discover` 出口里会多出 `192.168.1.255` 的定向广播 + 单播探测,设备同网段
直接二层应答,无需网关。扫到后改好 IP,再 `ip addr del` 撤掉辅助地址即可。

设备名 `dev_name` 兼容厂家工具写入的 GBK/ANSI 中文字节;写中文设备名时也按 GBK 编码。

## 批量配置

```yaml
# devices.yaml —— 按 DevID(MAC)匹配,IP 改完即变不能作主键
- match: {devid: "5a:4c:6f:73:cc:d6"}
  set:   {local_ip: 10.0.0.11, net_mask: 255.255.255.0, gateway: 10.0.0.1}
- match: {devid: "5a:4c:6f:73:cc:d7"}
  set:   {local_ip: 10.0.0.12}
```

```sh
zlan apply -f devices.yaml            # dry-run 预览 before->after,不写入
zlan apply -f devices.yaml --confirm  # 实际执行
```

## 输出与退出码

所有命令支持 `--json`(机器可读)。全局 flag:`-q/--quiet`、`-v/--verbose`、`--no-color`、`-y/--yes`、`--confirm`、`--timeout`、`--retries`。

| 码 | 含义 | 码 | 含义 |
|---|---|---|---|
| 0 | 成功 | 5 | 协议解析失败 |
| 2 | 用法错误 | 6 | 鉴权失败 |
| 3 | 设备未找到/无应答 | 7 | 批量部分失败 |
| 4 | 超时 | 130 | Ctrl-C 取消 |

## ⚠ 待真机验证

字节布局严格对齐两份官方文档 + 样例,但以下推定**尚未在真机验证**(详见 `SPEC.md` §17),首次对真机使用请留意并反馈:

1. `parity` 的 even/odd 编码(两份文档定义相反);
2. 串口单帧读写上限(当前保守分段 64 字节);
3. 写后是否有 ACK;保活/重连字节序(96/97);
4. 改参密码(`func_en.need_password`)的编码方式 —— 本期未实现密码写入。

IO 控制、中心服务器上报功能依赖未提供的外部文档,**不在本期范围**。

## 文档

- `SPEC.md` —— 完整协议规格、偏移表、命令语义、golden 向量、待验证清单。
- `docs/README.md` —— 官方说明书、旧版 CLI 说明与现场审计记录索引。
- `CLAUDE.md` / `AGENTS.md` —— Claude Code 与 Codex 共用的开发须知与协议铁坑(`AGENTS.md` 为软链)。
