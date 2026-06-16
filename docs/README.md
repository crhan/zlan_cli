# Reference Documents

This directory keeps source documents and field notes that are useful when
maintaining `zlan`.

## Official / Vendor References

- `reference/ZLAN7110M-WIFI串口服务器说明书.pdf`
  - Source: `/home/ruohanc/project/zlan/docs/ZLAN7110M-WIFI串口服务器说明书.pdf`
  - Useful for ZLAN7110M WiFi setup, ZLVirCOM behavior, Modbus TCP gateway setup,
    and the documented OTA firmware upgrade workflow.

## Project References

- `reference/legacy-zlanctl.md`
  - Source: `/home/ruohanc/project/zlan/docs/zlanctl.md`
  - Notes from the earlier Python `zlanctl`, including the two management
    channels and serial command mode notes.

## Field Notes

- `field-notes/2026-06-15-zlan-device-audit.md`
  - Source: `/home/ruohanc/project/zlan/reports/2026-06-15-zlan-device-audit.md`
  - Real-device observations from the local network, including firmware versions,
    response behavior, web-port checks, and configuration differences.

## Not Copied

The sibling `zlan` folder also contains pressure-transmitter, water-meter, CH340
driver, and serial-tool files. Those are external-device setup materials and are
not directly relevant to this CLI's protocol implementation.
