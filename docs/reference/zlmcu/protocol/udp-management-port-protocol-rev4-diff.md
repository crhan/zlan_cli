# UDP Management Protocol Rev.4 Diff Notes

Source: `udp-management-port-protocol-rev4.pdf`, provided on 2026-06-23.

The existing CLI model was based on the same ZL DUI 20100427.1.0 UDP protocol
family, but `SPEC.md` referenced Rev.3. Rev.4 adds the variable parameter area
method, which is the missing native path for WiFi configuration.

## What Did Not Change

- UDP management port remains `1092`.
- UDP frame remains `[5a 4c][cmd][167-byte param]`, total 170 bytes.
- `0x00`, `0x01`, `0x02`, `0x03`, and `0x04` command semantics match the current
  implementation.
- The fixed parameter block offsets from 0 through 114 still match the current
  `SPEC.md` and `internal/protocol/fields.go`.
- Offset `115..166` remains a 52-byte tail inside the same 167-byte parameter
  block. It is not an appended extension frame.

## Main Rev.4 Addition

Rev.4 documents `user_param@115` as a 52-byte TLV sequence:

```text
type: 1 byte
len:  1 byte
data: len bytes
...
type 0 terminates parsing
```

The parameter area must still remain exactly 52 bytes when written back through
the normal `0x02` whole-parameter write path.

Documented TLV types:

| Type | Meaning |
|---:|---|
| `0` | End marker |
| `1` | Function selection |
| `2` | WiFi SSID |
| `3` | WiFi channel / bridge / DHCP-server bits |
| `4` | WiFi password |
| `5` | WiFi encryption type and STA/AP mode |
| `6` | Multi-destination IP |
| `8` | Proxy server parameters |
| `9` | Serial TX byte count, read-only/status-like |
| `10` | Serial RX byte count, read-only/status-like |
| `255` | Custom |

## WiFi TLV Encoding

### SSID

TLV type `2`. The value is raw SSID bytes with no trailing `0x00`.

Example:

```text
02 04 74 65 73 74
```

means SSID `test`.

### Channel, Ethernet/WiFi Bridge, DHCP Server

TLV type `3`, length `1`.

```text
bit0..bit3 = WiFi channel number
bit6       = 1 means DHCP server disabled
bit7       = 1 means Ethernet/WiFi bridge enabled
```

Example from Rev.4:

```text
03 01 44
```

means channel `4`, Ethernet/WiFi bridge disabled, DHCP server disabled.

### Password

TLV type `4`. The value is raw password bytes with no trailing `0x00`.

Example:

```text
04 03 6b 65 79
```

means password `key`.

### Encryption And STA/AP Mode

TLV type `5`, length `1`.

```text
bit0..bit5 = encryption
bit6       = 1 means AP mode
bit7       = 1 means STA mode
```

Encryption values in Rev.4 TLV encoding:

| Value | Meaning |
|---:|---|
| `0` | none |
| `1` | WEP64 |
| `2` | TKIP |
| `4` | AES |
| `5` | WEP128 |
| `6` | auto |

Example from Rev.4:

```text
05 01 86
```

means STA mode plus auto encryption.

Another Rev.4 example writes:

```text
05 01 40
```

meaning AP mode plus no encryption.

## Difference From ZLDevManage DLL IDs

The SDK header exposes higher-level DLL parameter IDs such as
`PARAM_WIFI_SSID=350` and `PARAM_WIFI_KEY=352`. Those are not the TLV type
numbers documented by Rev.4.

The likely relationship is:

| SDK/DLL parameter | Rev.4 TLV |
|---|---|
| `PARAM_WIFI_SSID` | type `2` |
| `PARAM_WIFI_CHANNEL`, `PARAM_WIFI_DHCP_SERVER`, `PARAM_WIFI_ETH_BRIDGE` | type `3` packed bits |
| `PARAM_WIFI_KEY` | type `4` |
| `PARAM_WIFI_KEY_TYPE`, `PARAM_WIFI_STA_AP` | type `5` packed bits |

The encryption enum also differs between the SDK UI order and the Rev.4 TLV
wire values. A native Go implementation must use the Rev.4 TLV values on the
wire.

## Implementation Impact

The fixed-field registry still treats `user_param` as non-settable opaque data,
which remains the safe default for generic `set field=value`.

WiFi support should use the dedicated parser/builder in
`internal/protocol/user_param.go` rather than turning WiFi into ordinary
fixed-offset fields. The parser/builder should continue to:

- Preserve unknown TLVs and their order when possible.
- Drop or ignore read-only counters `9` and `10` on write unless preserving a
  read snapshot exactly.
- Validate the rebuilt TLV byte stream fits in 52 bytes.
- Terminate with type `0` and zero-fill the remaining bytes.
- Use deterministic last-one-wins parsing for duplicate WiFi TLVs and rebuild a
  de-duplicated WiFi set before exposing CLI writes.

The high-level CLI shape is native UDP:

```text
zlan wifi get <target>
zlan wifi set <target> ssid=... key=... mode=sta crypt=auto channel=...
```

Under the hood this should be the normal read-modify-write path:

1. Read full parameters with UDP `0x04`.
2. Parse `user_param@115`.
3. Modify WiFi TLVs.
4. Rebuild exactly 52 bytes.
5. Write the full 167-byte parameter block with UDP `0x02`.

Serial support may be possible by writing the same offset `115..166` with the
serial persistent-write command, but the new PDF only documents UDP examples.
