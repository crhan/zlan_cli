# ZLAN 设备只读探索报告

日期: 2026-06-15

## 结论摘要

当前家庭网络里通过 ZLAN UDP 1092 管理协议发现 **4 台 ZLAN 设备**:

| IP | 名称 | DevID | 固件 | 工作模式 | 串口 | 转化协议 | 连接状态 |
|---|---|---|---|---|---|---|---|
| 192.168.15.40 | 热水循环 | 04:ee:e8:1a:06:09 | 1.396 | tcp-server | 9600/8/none | transparent | connected |
| 192.168.15.41 | 1401电表 | 04:ee:e8:1a:06:05 | 1.396 | tcp-client | 9600/8/odd | transparent | disconnected |
| 192.168.15.42 | 1401新风 | 28:52:75:f3:8e:9e | 1.528 | tcp-server | 9600/8/none | transparent | disconnected |
| 192.168.15.43 | 1803新风 | 28:52:20:39:ae:2f | 1.473 | tcp-server | 9600/8/none | transparent | connected |

关键判断:

- 这 4 台目前全部是 **transparent 透明传输**,不是 Modbus TCP <-> Modbus RTU 转换模式。
- 如果要接本项目的压力变送器/水表总线,现状没有一台已经处于目标配置。目标应接近: `tcp-server + local_port 502 + app_protocol modbus + 9600/8/none`。
- `192.168.15.42` 和 `192.168.15.43` 最接近可改造成 Modbus 网关:固件较新、串口是 `9600/8/none`、TCP 服务端端口和 Web 80 端口实测可连。
- `192.168.15.41` 串口是 `9600/8/odd`,且工作模式是 `tcp-client`,当前连接未建立;不适合直接接本项目的 9600/8/N/1 Modbus 总线。
- UDP 广播应答不稳定,单轮扫描可能漏设备。多轮合并后才拿齐 4 台。

## 已跑读功能

### `discover`

命令:

```bash
UV_CACHE_DIR=.uv-cache UV_LINK_MODE=copy uv run zlanctl discover --target 192.168.15.255 --bind 192.168.14.107 --timeout 12 --json --raw
```

结果:

- 实测可用。
- 多轮扫描分别返回 2/3/4 台,最终合并确认 4 台。
- 设备名已按 GBK 正常解码;之前的 `����` 是厂家工具写入 GBK 名称,不是设备名损坏。

### `show`

逐台运行:

```bash
UV_CACHE_DIR=.uv-cache UV_LINK_MODE=copy uv run zlanctl show <ip> --bind 192.168.14.107 --timeout 5 --json
```

结果:

| IP | 结果 |
|---|---|
| 192.168.15.40 | no reply |
| 192.168.15.41 | no reply |
| 192.168.15.42 | no reply |
| 192.168.15.43 | no reply |

补充实验:不指定 `--bind` 对 `192.168.15.40` 再试一次,仍无回复。

判断:这些设备对 UDP `0x00` 广播查询会回包,但对 UDP `0x04` 一对一查询不回包。原因未确认;可能是固件/配置/网络策略差异。当前应以 `discover` 多轮广播作为主读取方式。

### `uart-status` / `uart-read`

本机串口枚举:

```text
/dev/cu.Bluetooth-Incoming-Port
/dev/cu.debug-console
```

未发现 CH340/USB 转串口设备。没有真实串口链路时,不能对 ZLAN 执行串口读数。只验证了命令帧 dry-run:

```bash
uv run zlanctl uart-status COM3 --dry-run --json
# ed f2 a3 56 ca db 91 84 b0 d7 00 3d 01

uv run zlanctl uart-read COM3 --offset 0x00 --length 0xa7 --dry-run --json
# ed f2 a3 56 ca db 91 84 b0 d7 00 00 a7
```

判断:串口读功能已验证帧格式,但本次没有真实串口硬件,所以没有设备侧返回数据。

## 逐台设置详情

敏感说明:参数区 `key` 字段均非空,可能是管理密码/Key。报告只记录 `key_present=true`,不记录具体内容。

### 192.168.15.40 热水循环

| 项 | 值 |
|---|---|
| DevID | 04:ee:e8:1a:06:09 |
| Local IP | 192.168.15.40 |
| Netmask | 255.255.254.0 |
| Gateway | 192.168.15.1 |
| DHCP | false |
| DNS | 192.168.15.1 |
| Work mode | tcp-server |
| Local port | 4196 |
| Dest IP | 192.168.15.15 |
| Dest string | mqtt.crhan.com |
| Dest port | 1883 |
| Dest dynamic | true |
| App protocol | transparent |
| Serial | 9600/8/none |
| Gap time | 27 |
| Packing length | 1024 |
| Flow control | false |
| Status | connected |
| Keep alive | 12 |
| Reconnect | 60 |
| Web port | 51744 |
| Firmware | 1.396 |
| UDP group IP | 230.90.76.1 |
| Key present | true |
| Frame start/end rules | disabled |
| Func enable | 0 |
| User/reserved tail | non-empty |

端口实测:

| 端口 | 结果 |
|---|---|
| 4196 | open |
| 51744 | connection refused |

洞察:

- 名称和目标域名指向热水循环的 MQTT/透明传输场景。
- `web_port=51744` 但端口拒绝连接;结合固件 1.396,不能确认是配置异常还是该型号/固件不提供 Web。
- `status_connected=true` 且 4196 open,说明当前可能已有客户端连接或设备认为链路已建立。

### 192.168.15.41 1401电表

| 项 | 值 |
|---|---|
| DevID | 04:ee:e8:1a:06:05 |
| Local IP | 192.168.15.41 |
| Netmask | 255.255.255.0 |
| Gateway | 192.168.15.1 |
| DHCP | false |
| DNS | 192.168.15.1 |
| Work mode | tcp-client |
| Local port | 4196 |
| Dest IP/String | 192.168.15.15 |
| Dest port | 1883 |
| Dest dynamic | true |
| App protocol | transparent |
| Serial | 9600/8/odd |
| Gap time | 27 |
| Packing length | 1024 |
| Flow control | false |
| Status | disconnected |
| Keep alive | 12 |
| Reconnect | 60 |
| Web port | 47176 |
| Firmware | 1.396 |
| UDP group IP | 230.90.76.1 |
| Key present | true |
| Frame start/end rules | disabled |
| Func enable | 0 |
| User/reserved tail | non-empty |

端口实测:

| 端口 | 结果 |
|---|---|
| 4196 | timeout |
| 47176 | timeout |

洞察:

- 这是 TCP client 模式,所以本机连它的 local port 超时不意外。
- `status_connected=false`,目标 `192.168.15.15:1883` 当前没有建立连接。
- 串口奇校验 `odd` 是明显特殊配置,不要把它接到本项目 9600/8/N/1 总线上。

### 192.168.15.42 1401新风

| 项 | 值 |
|---|---|
| DevID | 28:52:75:f3:8e:9e |
| Local IP | 192.168.15.42 |
| Netmask | 255.255.254.0 |
| Gateway | 192.168.15.1 |
| DHCP | false |
| DNS | 192.168.15.1 |
| Work mode | tcp-server |
| Local port | 4196 |
| Dest IP/String | 192.168.15.15 |
| Dest port | 1883 |
| Dest dynamic | true |
| App protocol | transparent |
| Serial | 9600/8/none |
| Gap time | 3 |
| Packing length | 1300 |
| Flow control | false |
| Status | disconnected |
| Keep alive | 12 |
| Reconnect | 60 |
| Web port | 80 |
| Firmware | 1.528 |
| UDP group IP | 230.90.76.1 |
| Key present | true |
| Frame start/end rules | disabled |
| Func enable | 0 |
| User/reserved tail | non-empty |

端口实测:

| 端口 | 结果 |
|---|---|
| 4196 | open |
| 80 | open |

洞察:

- 固件最新,Web 80 可连,串口参数也是 9600/8/N;如果要选一台改造成 Modbus 网关,它是候选。
- 当前仍是透明传输,不是 Modbus 转换。接 Modbus Poll 读压力/水表前必须改 `app_protocol=modbus`,通常还要改本地端口到 502。
- `status_connected=false` 表示当前没有活跃 TCP 连接,对改造/维护影响较小。

### 192.168.15.43 1803新风

| 项 | 值 |
|---|---|
| DevID | 28:52:20:39:ae:2f |
| Local IP | 192.168.15.43 |
| Netmask | 255.255.254.0 |
| Gateway | 192.168.15.1 |
| DHCP | false |
| DNS | 192.168.15.1 |
| Work mode | tcp-server |
| Local port | 4196 |
| Dest IP/String | 192.168.1.154 |
| Dest port | 4196 |
| Dest dynamic | true |
| App protocol | transparent |
| Serial | 9600/8/none |
| Gap time | 3 |
| Packing length | 1300 |
| Flow control | false |
| Status | connected |
| Keep alive | 12 |
| Reconnect | 60 |
| Web port | 80 |
| Firmware | 1.473 |
| UDP group IP | 230.90.76.1 |
| Key present | true |
| Frame start/end rules | disabled |
| Func enable | 0 |
| User/reserved tail | non-empty |

端口实测:

| 端口 | 结果 |
|---|---|
| 4196 | open |
| 80 | open |

洞察:

- 也是可维护候选:Web 80 open,串口 9600/8/N。
- `dest=192.168.1.154` 和本机所在 `192.168.14/23` 不同网段,但该设备是 tcp-server,这个目的地址未必实际使用。
- `status_connected=true` 表示当前可能有业务客户端连接。改配置前要确认不会影响现有 1803 新风链路。

## 横向洞察

### 1. 这些不是当前项目的 Modbus 网关配置

全部设备 `app_protocol=transparent`,而项目目标是 ZLAN7110M 作为 Modbus TCP 到 RTU 网关。透明传输模式下,Modbus TCP 客户端不能直接按网关方式读 RTU 从机。

目标配置应至少包括:

- `work_mode=tcp-server`
- `local_port=502`
- `app_protocol=modbus`
- `serial=9600/8/none`

当前最接近的是 `192.168.15.42` 和 `192.168.15.43`,但它们仍需改协议和端口。

### 2. 设备分两代

- `04:ee:e8:*` 两台固件 1.396,名称/高级参数形态相似,Web 端口字段异常且实测不可连。
- `28:52:*` 两台固件 1.473/1.528,Web 80 可连,参数更标准,更适合作为后续维护对象。

### 3. UDP 发现不能只跑一轮

同一命令多轮返回数量不同。已观察到 2/3/4 台。报告和自动化脚本应该采用多轮合并,否则会漏设备。

### 4. `show` 单播读当前不可用

4 台都不响应 `0x04` 一对一查询。这个不是“没设备”,因为广播查询能拿到完整参数。后续 CLI 可以考虑增加 `discover --repeat N` 或者 `show --via-broadcast <ip>` 来降低误判。

### 5. 不要覆盖 User Param / reserved tail

4 台的 115 字节后的用户/保留区域均非空。这通常意味着高级功能、注册包、心跳包或厂家自定义内容在用。除非确认用途,不要用串口命令或 UDP 参数写入整段覆盖这块。

### 6. `1401电表` 是高风险异类

它同时具备:

- tcp-client
- disconnected
- 9600/8/odd
- Web 不通

如果这是电表链路,先不要动。任何统一改串口参数的操作都可能直接破坏它的通信。

## 建议下一步

1. 如果目标是给压力变送器/水表项目选一台 ZLAN 网关,优先确认 `192.168.15.42` 是否空闲。
2. 改造前先在 Web UI 或 ZLVircom 里备份完整参数,尤其是 `key` 和用户参数区。
3. 改造命令应 dry-run 后人工确认,目标大致为:

```bash
uv run zlanctl set 192.168.15.42 --profile modbus-tcp-rtu --dry-run
```

4. 不建议动 `192.168.15.41`,除非确认 1401 电表当前确实废弃或另有备份。

