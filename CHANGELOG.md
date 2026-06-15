# Changelog

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
