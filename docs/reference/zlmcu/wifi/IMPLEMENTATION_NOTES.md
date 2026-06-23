# WiFi Configuration Implementation Notes

This note is scoped to material that can help implement WiFi configuration in
the `zlan` CLI. It separates confirmed data from remaining protocol gaps.

## Current CLI Protocol Model

The current `zlan` UDP and serial implementation is based on the 167-byte
parameter block documented in `SPEC.md`. Rev.4 of the UDP protocol confirms that
WiFi settings live inside the 52-byte `user_param` area at offset 115, encoded as
TLV records.

Do not add WiFi support by guessing fixed offsets. Implement it as a parser and
builder for `Param[115:167]`.

## Native UDP TLV Encoding

Source files:

- `../protocol/udp-management-port-protocol-rev4.pdf`
- `../protocol/udp-management-port-protocol-rev4-diff.md`

Rev.4 documents the 52-byte `user_param@115` area as TLV:

```text
type: 1 byte
len:  1 byte
data: len bytes
```

WiFi-related TLV types:

| Type | Meaning |
|---:|---|
| `2` | SSID, raw bytes, no trailing zero |
| `3` | Channel, Ethernet/WiFi bridge, DHCP server bits |
| `4` | Password, raw bytes, no trailing zero |
| `5` | Encryption type and STA/AP mode |

Type `3`, length `1`:

```text
bit0..bit3 = channel
bit6       = 1 means DHCP server disabled
bit7       = 1 means Ethernet/WiFi bridge enabled
```

Type `5`, length `1`:

```text
bit0..bit5 = encryption
bit6       = 1 means AP mode
bit7       = 1 means STA mode
```

Encryption values on the Rev.4 wire format:

| Value | Meaning |
|---:|---|
| `0` | none |
| `1` | WEP64 |
| `2` | TKIP |
| `4` | AES |
| `5` | WEP128 |
| `6` | auto |

## Direct Parameter API Evidence

The ZLDevManage SDK exposes WiFi settings as extended parameter IDs. These are
DLL-level parameter IDs, not TLV type numbers and not offsets in the 167-byte
block.

Source files:

- `../zldevmanage-dll.h`
- `../zldevmanage-winp2p-dev-library.pdf`
- `../sdk/zldevmanage-x64/ZLUseDevManage/DlgParam.cpp`

Confirmed parameter IDs:

| Parameter | ID | Type | Meaning |
|---|---:|---|---|
| `PARAM_WIFI_SSID` | 350 | string | AP SSID when in AP mode, router SSID when in STA mode |
| `PARAM_WIFI_CHANNEL` | 351 | int | AP channel. SDK demo writes 1..11 and reads value minus 1 into the combo box |
| `PARAM_WIFI_KEY` | 352 | string | WiFi password/key |
| `PARAM_WIFI_KEY_TYPE` | 353 | int | Encryption type |
| `PARAM_WIFI_STA_AP` | 354 | int | WiFi working mode |
| `PARAM_WIFI_DHCP_SERVER` | 355 | int | DHCP server switch |
| `PARAM_WIFI_ETH_BRIDGE` | 356 | int | Ethernet/WiFi bridge switch |

Confirmed enum values:

| Field | Values |
|---|---|
| `PARAM_WIFI_STA_AP` | `0=AP`, `1=STA` |
| `PARAM_WIFI_KEY_TYPE` | `0=NONE`, `1=WEP64`, `2=WEP128`, `3=TKIP`, `4=AES`, `5=AUTO`; the SDK UI text has `WEB64/WEB128`, which appears to be a typo |
| `PARAM_WIFI_DHCP_SERVER` | `0=Disable`, `1=Enable` |
| `PARAM_WIFI_ETH_BRIDGE` | `0=Disable`, `1=Enable` |
| `PARAM_WIFI_CHANNEL` | `1..11` in the SDK call path |

ZLDevManage write flow, from `DlgParam.cpp`:

```cpp
(*m_pZLDM_SetDevParamString)(g_pCurSelID, m_WifiSSID, PARAM_WIFI_SSID);
(*m_pZLDM_SetDevParamString)(g_pCurSelID, m_WifiKey, PARAM_WIFI_KEY);
(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_WifiChannel.GetCurSel()+1, PARAM_WIFI_CHANNEL);
(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_WifiDHCPServer.GetCurSel(), PARAM_WIFI_DHCP_SERVER);
(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_WifiEthWifiBridge.GetCurSel(), PARAM_WIFI_ETH_BRIDGE);
(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_WifiKeyType.GetCurSel(), PARAM_WIFI_KEY_TYPE);
(*m_pZLDM_SetDevParamInt)(g_pCurSelID, m_WifiSTAAP.GetCurSel(), PARAM_WIFI_STA_AP);
```

After setting parameters the demo calls `ZLDM_SetDevParamExcute`, which writes
the queued settings to the device. The SDK document says this applies the
parameter changes and the device restarts.

Important difference: the SDK UI enum order is not identical to the Rev.4 TLV
wire values. A native Go implementation should use the Rev.4 TLV values on the
wire and treat the SDK IDs as corroborating API evidence only.

## `param.txt` Route

The ZLAN7110M manual documents a configuration-file path for features outside
the normal parameter dialog.

Confirmed keys:

```ini
AP_STA=1
MDNS=1
M_TBD=zlan-device-name
M_TXT_NUM=5
M_TXT1=manufacturer=Phoenix
SET_DEV_NAME=1
```

Observed meanings:

- `AP_STA=1` enables AP+STA mode.
- `AP_STA=0` disables AP+STA mode.
- `MDNS=1` enables mDNS.
- `SET_DEV_NAME=1` makes `M_TBD` update the device name and DHCP client name.

The manual says the file is downloaded through ZLVirCOM's firmware/config file
download workflow. It does not document that file-transfer wire protocol.

## `wifi.txt` Route

Several WiFi product manuals document `wifi.txt` for multi-WiFi fallback
configuration. This is useful for a future CLI command that renders or uploads
WiFi fallback profiles.

Example format from the manuals:

```ini
DEFAULT_WIFI_TIME=10
WIFI_CONFIG_COUNT=2

WIFI_MODE1=STA
WIFI_SSID1=TP-LINK_2312
WIFI_CRYPT1=AUTO
WIFI_KEY1=12345678
WIFI_BRIDGE1=0
WIFI_DHCP1=0
WIFI_TIME1=10

WIFI_MODE2=AP
WIFI_SSID2=TEMP_AP
WIFI_CRYPT2=NONE
WIFI_IP2=192.168.1.200
WIFI_TIME2=10
```

Confirmed keys and values:

| Key | Values / meaning |
|---|---|
| `DEFAULT_WIFI_TIME` | Seconds to use the default ZLVirCOM WiFi parameter set before moving to `wifi.txt` entries |
| `WIFI_CONFIG_COUNT` | Number of extra WiFi entries in the file, excluding the default ZLVirCOM set |
| `WIFI_MODEn` | `STA` or `AP` |
| `WIFI_SSIDn` | Router SSID in STA mode, device AP SSID in AP mode |
| `WIFI_CRYPTn` | Usually `AUTO`; manuals also mention `NONE`, `WEP64`, `WEP128`, `AES`, `TKIP` |
| `WIFI_KEYn` | WiFi password; omit or leave unused for `NONE` |
| `WIFI_BRIDGEn` | `0` off, `1` on; omitted default is off |
| `WIFI_DHCPn` | DHCP server switch. If omitted, AP defaults on and STA defaults off |
| `WIFI_IPn` | Static IP used by AP fallback examples |
| `WIFI_TIMEn` | Required dwell time before trying the next entry when connection fails |

Manual notes relevant to a CLI:

- Device firmware loads `wifi.txt` on boot; changing the file requires reboot or
  power cycle.
- Firmware update can erase `wifi.txt`.
- DEF/reset mode ignores `wifi.txt`.
- If WiFi connects successfully but TCP connection does not, the device does not
  switch to the next WiFi entry.
- Devices only support 2.4 GHz channels 1 through 11. A router on channel 12 or
  higher can prevent STA connection.
- For STA mode troubleshooting, DHCP server and Ethernet/WiFi bridge should
  normally be disabled.

The manuals again describe ZLVirCOM web-directory/config download, not the
underlying upload packets.

## Implementation Consequences

There are two realistic implementation paths:

1. Native direct WiFi parameter setting:
   - Use the `user_param@115` TLV parser/builder in
     `internal/protocol/user_param.go`.
   - Use the normal UDP read-modify-write path: query with `0x04`, rebuild TLVs,
     write full 167-byte parameter block with `0x02`.
   - The existing fixed-field registry is insufficient by itself because WiFi is
     a variable TLV substructure.
   - Minimal CLI entry points are `zlan wifi get` and `zlan wifi set`.

2. File-based WiFi profile support:
   - The CLI can safely generate `wifi.txt` or `param.txt` from structured flags.
   - Uploading those files still needs the ZLVirCOM file-download protocol, unless
     the workflow remains "generate the file for manual upload".

## Missing Material

Still not found in public ZLMCU pages or downloaded archives as of 2026-06-23:

- The standalone `串口修改参数及硬件 TCPIP 协议栈` document with any WiFi
  serial command encoding.
- The wire protocol for ZLVirCOM firmware/config/web-directory file download.

Best next evidence to collect:

- Confirm `zlan wifi set` behavior on a real ZLAN7110M or related WiFi device.
- Packet-capture ZLVirCOM or ZLDevManage while changing `PARAM_WIFI_*` to
  confirm real-device behavior and SDK enum mapping.
- Packet-capture ZLVirCOM while uploading `wifi.txt` or `param.txt`.
