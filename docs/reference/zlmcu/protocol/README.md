# ZLMCU Protocol References

This directory keeps protocol-level documents that are directly relevant to the
`zlan` CLI implementation.

## Files

| File | Source | Notes |
|---|---|---|
| `udp-management-port-protocol-rev4.pdf` | Provided by the user on 2026-06-23 | `卓岚联网产品 UDP 管理端口协议`, ZL DUI 20100427.1.0, Rev.4 |
| `udp-management-port-protocol-rev4.txt` | Extracted with `pdftotext -layout` from the PDF | Grep-friendly text extraction |
| `udp-management-port-protocol-rev4-diff.md` | Local analysis | Differences from the previously modeled Rev.3 behavior |

## Key Point

Rev.4 keeps the same UDP frame shape and 167-byte parameter block:

```text
[0x5a 0x4c][cmd 1B][param 167B]
```

The important new material is the 52-byte variable parameter area at parameter
offset 115. Rev.4 documents that area as a compact TLV sequence and explicitly
uses it for WiFi settings.
