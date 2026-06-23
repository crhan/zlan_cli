# ZLMCU Vendor Reference Bundle

Fetched from official ZLMCU/Zorlan pages on 2026-06-23. Filenames here are
normalized to ASCII; original titles and source URLs are recorded below.

For the WiFi configuration work, start with
`wifi/IMPLEMENTATION_NOTES.md` and
`protocol/udp-management-port-protocol-rev4-diff.md`. The Rev.4 UDP protocol
document fills the previous wire-protocol gap by documenting the 52-byte
`user_param` TLV area used for WiFi settings.

## Product And Core Manuals

| File | Official title / purpose | Source |
|---|---|---|
| `zlan7110m-user-manual.pdf` | ZLAN7110M user manual, including WiFi AP/STA/AP+STA setup, Modbus, MQTT/JSON, HTTPD, mDNS, UDP management overview, and serial parameter modification overview | `https://www.zlmcu.com/download/ZLAN7110M.pdf` |
| `zlan7110m-dimensions.pdf` | ZLAN7110M dimensions drawing | `https://www.zlmcu.com/download/7110M尺寸图.pdf` |
| `zlan-ce-certificate-2023.pdf` | ZLAN CE certificate linked from the ZLAN7110M product page | `https://www.zlmcu.com/download/ZLAN_CE_Certificate_2023.pdf` |
| `serial-server-user-manual.pdf` | 联网产品使用指南 | `https://www.zlmcu.com/download/serial_server_user_manual.pdf` |
| `serial-server-quick-start.pdf` | 串口服务器快速上手指南 | `https://www.zlmcu.com/download/SerialServerQuickStart.pdf` |
| `firmware-update.pdf` | 固件升级方法 | `https://www.zlmcu.com/download/FirmwareUpdate.pdf` |

## Protocol And Feature References

| File | Official title / purpose | Source |
|---|---|---|
| `mqtt-and-json-to-modbus-gateway.pdf` | 卓岚 MQTT 和 JSON 转 Modbus 网关用法 | `https://www.zlmcu.com/FAQ/MQTT%20and%20JSON%20to%20Modbus%20Gateway.pdf` |
| `modbus-gateway-user-manual.pdf` | Modbus 网关使用指南 | `https://www.zlmcu.com/download/Modbus_GateWay_UM.pdf` |
| `zlvircom-user-manual.pdf` | ZLVirCom 用户手册 | `https://www.zlmcu.com/download/Zorlan_ZLVirCom_User_manual.pdf` |
| `socket-testdlg-user-manual.pdf` | SocketTestDlg 用户手册 | `https://www.zlmcu.com/download/Zorlan_SocketTestDlg_Usr_Manual.pdf` |
| `zldevmanage-winp2p-dev-library.pdf` | 卓岚 WinP2p 和设备管理开发库, extracted from `tools/zldevmanage-x64.zip` | `https://www.zlmcu.com/download/ZLDevManage_x64.zip` |
| `zldevmanage-dll.h` | ZLDevManage DLL header, extracted from `tools/zldevmanage-x64.zip` | `https://www.zlmcu.com/download/ZLDevManage_x64.zip` |
| `zldevmanage-readme.txt` | ZLDevManage example ReadMe, extracted from `tools/zldevmanage-x64.zip` | `https://www.zlmcu.com/download/ZLDevManage_x64.zip` |

## Protocol References

| File | Purpose | Source |
|---|---|---|
| `protocol/udp-management-port-protocol-rev4.pdf` | `卓岚联网产品 UDP 管理端口协议`, Rev.4 | Provided by the user on 2026-06-23 |
| `protocol/udp-management-port-protocol-rev4.txt` | Text extraction for search/diff | Extracted locally from the PDF |
| `protocol/udp-management-port-protocol-rev4-diff.md` | Local analysis of differences relevant to `zlan` | Created from the Rev.4 PDF |

## WiFi Implementation Subset

| File | Purpose |
|---|---|
| `wifi/IMPLEMENTATION_NOTES.md` | WiFi CLI implementation notes |
| `wifi/README.md` | WiFi-specific source index |
| `wifi/*.pdf` | WiFi product manuals and troubleshooting docs |
| `protocol/udp-management-port-protocol-rev4.pdf` | Native UDP TLV encoding for WiFi settings in `user_param@115` |
| `sdk/zldevmanage-x64/ZLUseDevManage/DlgParam.cpp` | Vendor demo source showing `PARAM_WIFI_*` get/set calls |
| `sdk/zldevmanage-x64/ZLUseDevManage/DlgParam.h` | Vendor demo dialog fields |
| `sdk/zldevmanage-x64/ZLUseDevManage/resource.h` | Vendor demo WiFi control IDs |
| `sdk/zldevmanage-x64/ZLUseDevManage/ZLUseDevManageDlg.*` | Vendor demo app context for loading and using the SDK |

## Saved Official HTML Pages

The vendor site exposes some feature docs only as HTML pages, so the source
HTML is archived under `pages/`.

| File | Purpose | Source |
|---|---|---|
| `pages/products-zlan7110m.html` | ZLAN7110M product page and product-linked downloads | `https://www.zlmcu.com/products_ZLAN7110M.htm` |
| `pages/products-wifi.html` | WiFi product category and related WiFi serial-server docs | `https://www.zlmcu.com/products_wifi.htm` |
| `pages/download.html` | Main ZLMCU downloads page | `https://www.zlmcu.com/download.htm` |
| `pages/usage-of-mqtt-gateway.html` | MQTT 网关的使用方法 | `https://www.zlmcu.com/document/Usage_of_MQTT_Gateway.html` |
| `pages/configurable-modbus-gateway-zlmb.html` | 可配置 Modbus 网关 / ZLMB usage | `https://www.zlmcu.com/document/Configurable_Modbus_gateway_ZLMB.html` |
| `pages/tech-dll-demo.html` | 设备管理函数库 DLL usage; notes that the DLL is based on the Zorlan UDP management-port protocol | `https://www.zlmcu.com/document/tech_dll_demo.html` |
| `pages/tcp-debug-tools.html` | SocketTestDlg download page | `https://www.zlmcu.com/document/tcp_debug_tools.html` |
| `pages/com-debug-tools.html` | ZLComDebug download page | `https://www.zlmcu.com/document/com_debug_tools.html` |

## Tool Archives

Stored under `tools/` for reproducibility of the vendor setup/debug workflow.

| File | Source |
|---|---|
| `tools/zlvircom.zip` | `https://www.zlmcu.com/download/ZLVirCom.zip` |
| `tools/zlvircom-green.zip` | `https://www.zlmcu.com/download/ZLVirComs.zip` |
| `tools/zlvircom-msi.zip` | `https://www.zlmcu.com/download/ZLVircom_msi.zip` |
| `tools/zldevmanage.zip` | `https://www.zlmcu.com/download/ZLDevManage.zip` |
| `tools/zldevmanage-v1.41.rar` | `https://www.zlmcu.com/download/ZLDevManageV1.41.rar` |
| `tools/zldevmanage-x64.zip` | `https://www.zlmcu.com/download/ZLDevManage_x64.zip` |
| `tools/socket-test.zip` | `https://www.zlmcu.com/download/SocketTest.zip` |
| `tools/zlcomdebug.zip` | `https://www.zlmcu.com/download/Comdebug.zip` |

## Missing Or Not Publicly Linked

- The 7110M manual references `http://zlmcu.com/download/Configurable_Modbus_gateway_ZLMB.pdf`, but that URL returned 404 on 2026-06-23. The current public ZLMB material is saved as `pages/configurable-modbus-gateway-zlmb.html`.
- The standalone `卓岚联网产品 UDP 管理端口协议` document was not publicly linked from the pages above, but Rev.4 was provided by the user and is now saved under `protocol/`.
- The standalone `串口修改参数及硬件 TCPIP 协议栈`, `卓岚 httpd 客户端通信方式`, and `卓岚 mDNS 功能用法` documents were referenced by the 7110M manual or implied by feature sections, but no public direct download link was found from the official pages checked on 2026-06-23.
