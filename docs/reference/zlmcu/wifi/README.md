# WiFi Implementation References

This directory contains the ZLMCU/Zorlan documents that are specifically useful
for implementing WiFi configuration support in `zlan`.

Read `IMPLEMENTATION_NOTES.md` first. It summarizes the usable API values,
file formats, and remaining wire-protocol gaps.

## Manuals

| File | Source | Implementation value |
|---|---|---|
| `../zlan7110m-user-manual.pdf` | `https://www.zlmcu.com/download/ZLAN7110M.pdf` | ZLAN7110M WiFi AP/STA settings, AP+STA `param.txt`, mDNS `param.txt`, UDP management and serial-parameter references |
| `zlan7104-user-manual.pdf` | `https://www.zlmcu.com/download/ZLAN7104.pdf` | WiFi settings and `wifi.txt` multi-profile format |
| `zlan7142-user-manual.pdf` | `https://www.zlmcu.com/download/ZLAN7142.pdf` | WiFi settings and `wifi.txt` multi-profile format |
| `zlan7146-user-manual.pdf` | `https://www.zlmcu.com/download/ZLAN7146.pdf` | Newer WiFi product behavior and `wifi.txt` format |
| `zlsn7004-user-manual.pdf` | `https://www.zlmcu.com/download/ZLSN7004.pdf` | WiFi settings and `wifi.txt` format for module-class product |
| `zlsn7046-user-manual.pdf` | `https://www.zlmcu.com/download/ZLSN7046.pdf` | WiFi settings and `wifi.txt` format |
| `zlsn7046t-user-manual.pdf` | `https://www.zlmcu.com/download/ZLSN7046T.pdf` | WiFi settings and `wifi.txt` format |
| `zlan7104-cannot-connect-to-wifi.pdf` | `https://www.zlmcu.com/FAQ/ZLAN7104_can't_connect_to_WIFI.pdf` | Troubleshooting constraints: exact SSID/key, DHCP server off, bridge off, channel 1..11 |
| `zlan7100-problems.pdf` | `https://www.zlmcu.com/FAQ/problem7100.pdf` | Background troubleshooting, lower direct implementation value |

## SDK Evidence

The useful SDK files are kept outside this directory because they belong to the
ZLDevManage archive:

| File | Source |
|---|---|
| `../zldevmanage-dll.h` | Extracted from `../tools/zldevmanage-x64.zip` |
| `../zldevmanage-winp2p-dev-library.pdf` | Extracted from `../tools/zldevmanage-x64.zip` |
| `../sdk/zldevmanage-x64/ZLUseDevManage/DlgParam.cpp` | Extracted vendor demo source showing WiFi read/write calls |
| `../sdk/zldevmanage-x64/ZLUseDevManage/DlgParam.h` | Extracted vendor demo source |
| `../sdk/zldevmanage-x64/ZLUseDevManage/resource.h` | Extracted vendor demo resource IDs |

The SDK confirms `PARAM_WIFI_*` values and enums, but not the underlying packet
format. The packet format is now documented separately by the Rev.4 UDP protocol
manual in `../protocol/udp-management-port-protocol-rev4.pdf`.

## Protocol Status

Now available:

- `../protocol/udp-management-port-protocol-rev4.pdf`: native UDP encoding for
  WiFi settings in the 52-byte `user_param@115` TLV area.

Still missing:

- `串口修改参数及硬件 TCPIP 协议栈` with WiFi serial command encoding.
- ZLVirCOM file-download wire protocol for uploading `wifi.txt` or `param.txt`.

The CLI now has the protocol-layer parser/builder and minimal `zlan wifi get`
/ `zlan wifi set` entry points for native WiFi writes through the normal UDP
`0x02` full-parameter write path.
