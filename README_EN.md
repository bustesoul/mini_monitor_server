# mini_monitor_server

[中文](./README.md) | [English](./README_EN.md)

Lightweight monitoring daemon for small servers (VPS, self-hosted nodes).

Single binary, minimal resource footprint, no database required.

## Features

- System metrics collection: CPU, Memory, Disk, Network
- Threshold-based alerting with configurable rules
- Telegram bot integration for remote queries
- Local HTTP API (`/report`, `/healthz`, `/history`)
- File-based storage, zero external dependencies

## Install

Download the latest release tarball for your architecture from the repository Releases page, then:

```bash
tar -xzf mini_monitor_server-*.tar.gz
cd mini_monitor_server-*/
sudo ./install.sh install
```

## Uninstall

```bash
sudo ./install.sh uninstall
```

## Manual Setup

1. `cp mini_monitor_server /usr/local/bin/`
2. `cp mini_monitor_server.example.yaml /etc/mini_monitor_server.yaml`
3. `cp mini_monitor_server.service /etc/systemd/system/`
4. Edit config: `vi /etc/mini_monitor_server.yaml`
5. Start: `systemctl enable --now mini_monitor_server`

## Commands

```bash
mini_monitor_server daemon  -c /etc/mini_monitor_server.yaml   # start daemon
mini_monitor_server report  -c /etc/mini_monitor_server.yaml   # print report
mini_monitor_server check   -c /etc/mini_monitor_server.yaml   # validate config
mini_monitor_server version                                     # print version
```

## File Locations

| File | Path |
|------|------|
| Binary  | `/usr/local/bin/mini_monitor_server` |
| Config  | `/etc/mini_monitor_server.yaml` |
| Service | `/etc/systemd/system/mini_monitor_server.service` |
| Data    | `/var/lib/mini_monitor_server/` |
