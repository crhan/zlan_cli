# Changelog

## Unreleased

## v0.6.0 - 2026-06-17

- Add `mqtt publish` / `mqtt bridge` to publish Modbus register reads to an MQTT broker.
- Support Home Assistant MQTT Discovery payloads for CLI-driven RS485 sensor bridges.

## v0.3.0 - 2026-06-16

- Add `reg read` / `reg write` for direct Modbus register access through ZLAN data channels.
- Auto-select Modbus TCP for `app_proto=modbus` and RTU-over-TCP for `app_proto=transparent`.
- Add manual `--mode`, `--data-host`, and `--data-port` overrides for field troubleshooting.
- Add ZLAN7110M reference docs and field notes under `docs/`, and include docs in release archives.

## v0.2.0 - 2026-06-16

- Add `copy`/`clone`/`cp` to copy writable ZLAN configuration from one online device to another.
- Add `export` and `import` for offline configuration backup, transfer, and restore.
- Preserve target device identity and read-only capability/status fields during copy/import.
- Support dry-run previews, field exclusions, and import-time overrides to avoid IP conflicts.

## v0.1.0 - 2026-06-15

Initial release.

- Manage ZLAN devices over UDP 1092 and serial command mode.
- Discover, inspect, read fields, set/tune parameters, reboot, monitor reports, list ports, and batch apply YAML manifests.
- Publish Linux and macOS binaries for amd64 and arm64.
