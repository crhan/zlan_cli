# zlanctl

`zlanctl` 是 ZLAN 参数管理命令行工具。它覆盖两条通道:

- UDP 1092 管理端口:发现、单设备参数读取、参数写入、重启、临时串口参数修改。
- 串口命令模式:通过 ZLAN 串口侧读取/写入参数字节、读取连接状态、V506+ 固件串口重启。

协议来源:

- `/Users/ruohan.chen/Downloads/UDP_port_protocol.pdf`
- `/Users/ruohan.chen/Downloads/Parameters_control_methods.pdf`

## 安装与运行

本项目用 `uv` 管理 Python 环境:

```bash
uv sync
uv run zlanctl --help
```

如果在受限 agent 环境里 `uv` 无法写默认缓存,用仓库本地缓存:

```bash
UV_CACHE_DIR=.uv-cache uv run zlanctl --help
```

## 常用命令

```bash
uv run zlanctl discover
uv run zlanctl show 192.168.1.200
uv run zlanctl show 192.168.1.200 --json
uv run zlanctl set 192.168.1.200 --profile modbus-tcp-rtu --yes
uv run zlanctl reboot 192.168.1.200 --yes
uv run zlanctl uart-status COM3
uv run zlanctl uart-read COM3 --offset 0x42 --length 0x1e
```

`set` 会先读取当前 167 字节参数区,只改命令行指定字段,再用 `0x02` 写回。设备收到 `0x02` 后会重启。没有 `--yes` 时会交互确认;非交互环境必须显式加 `--yes`。

`Device Name` 字段只有 10 字节,厂家工具可能按 GBK/ANSI 写中文名。CLI 读取时会按 UTF-8/GBK 兜底解码;写入 `--name` 时按 GBK 编码,中文名要控制在 9 字节以内并保留末尾 `NUL`。

## 项目内推荐配置

本仓库目标是让 ZLAN7110M 做 Modbus TCP 到 Modbus RTU 网关。推荐从这个 profile 起步:

```bash
uv run zlanctl set 192.168.1.200 --profile modbus-tcp-rtu --yes
```

该 profile 设置:

| 字段 | 值 |
|---|---|
| 工作模式 | `tcp-server` |
| 本地端口 | `502` |
| 转化协议 | `modbus` |
| 串口 | `9600/8/none` |

注意:这里的波特率码是 ZLAN 管理协议里的码位,`9600 = 4`;压力变送器自己的寄存器波特率码另算,不要混用。

## 串口命令模式

串口命令模式的识别流是:

```text
ed f2 a3 56 ca db 91 84 b0 d7
```

命令格式是 `识别流 + 命令类型 + 参数偏移 + 长度 + 可选内容`。常用命令类型:

| 类型 | CLI mode | 说明 |
|---|---|---|
| `0x00` | `uart-read` | 读参数字节 |
| `0x01` | `uart-write --mode volatile` | 写参数,通常不保存到 Flash |
| `0x03` | `uart-write --mode save` | 写参数并保存到 Flash |
| `0x07` | `uart-write --mode save-reboot` / `uart-reboot` | V506+ 固件保存并重启 |

串口命令必须用模块当前串口参数发送。项目推荐先假设 `9600/8/none/1`,不通再回到 ZLVircom/厂家工具确认。

示例:

```bash
uv run zlanctl uart-status COM3
uv run zlanctl uart-read COM3 --offset 0x1f --length 0x06
uv run zlanctl uart-write COM3 --offset 0x26 --data-hex "61 62 63 64 65 66 67 68 69 00" --mode save --yes
uv run zlanctl uart-reboot COM3 --yes
```

`uart-reboot` 使用 V506+ 文档里的 `07 1f 01 00` 方法。低于 V506 的固件要走 DNS 服务器交替写入那种旧方法,这个 CLI 暂不封装,避免误重启。

## 设计约束

- 输出数据走 stdout;错误和发送状态走 stderr。
- 所有命令支持 `--help`;也支持 `zlanctl help <command>`。
- `discover` 和 `show` 支持 `--json`,便于脚本接管。
- 写操作默认要确认,支持 `--dry-run` 先看 diff。
- 不打印参数区里的 `key` 字段,避免把可能的管理密码扫进日志。
