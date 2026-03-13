# mini_monitor_server

[中文](./README.md) | [English](./README_EN.md)

面向小型服务器（VPS、自托管节点）的轻量级监控守护进程。

单二进制、资源占用低、无需数据库。

## 功能特性

- 采集系统指标：CPU、内存、磁盘、网络
- 基于可配置规则的阈值告警
- 支持 Telegram Bot 远程查询
- 提供本地 HTTP API（`/report`、`/healthz`、`/history`、`/metrics`）
- 基于文件存储，零外部依赖
- 可选托管 `VictoriaMetrics single-node` 与 `vmagent`

## 安装

从仓库 Releases 页面下载适合你架构的发布压缩包，然后执行：

```bash
tar -xzf mini_monitor_server-*.tar.gz
cd mini_monitor_server-*/
sudo ./install.sh install-basic
```

发布包会一并携带固定版本的 `VictoriaMetrics single-node` 与 `vmagent` 二进制，安装脚本会默认将其安装到 `/usr/local/lib/mini_monitor_server/bin/`。

基础安装：

```bash
sudo ./install.sh install-basic
```

全量安装：

```bash
sudo ./install.sh install-full
```

- `install-basic`：写入标准配置文件，数据目录使用 `/var/lib/mini_monitor_server`
- `install-full`：写入启用 `VictoriaMetrics`/`vmagent` 的全量配置文件，数据目录使用 `/var/lib/mini_monitor_server`
- `install`：兼容旧命令，等价于 `install-basic`

## 卸载

```bash
sudo ./install.sh uninstall
```

`uninstall` 会直接清理基础安装和全量安装产生的二进制、配置、service 文件，以及 `/var/lib/mini_monitor_server` 数据目录。

## 手动安装

1. `cp mini_monitor_server /usr/local/bin/`
2. `cp mini_monitor_server.example.yaml /etc/mini_monitor_server.yaml`
3. `cp mini_monitor_server.service /etc/systemd/system/`
4. 编辑配置：`vi /etc/mini_monitor_server.yaml`
5. 启动服务：`systemctl enable --now mini_monitor_server`

## 命令

```bash
mini_monitor_server daemon  -c /etc/mini_monitor_server.yaml   # 启动守护进程
mini_monitor_server report  -c /etc/mini_monitor_server.yaml   # 输出报告
mini_monitor_server check   -c /etc/mini_monitor_server.yaml   # 校验配置
mini_monitor_server version                                     # 查看版本
```

## 文件位置

| 文件 | 路径 |
|------|------|
| 二进制 | `/usr/local/bin/mini_monitor_server` |
| 配置文件 | `/etc/mini_monitor_server.yaml` |
| Service 文件 | `/etc/systemd/system/mini_monitor_server.service` |
| 数据目录 | `/var/lib/mini_monitor_server/` |

## 配置说明

- `history.default_days`：`/history/disk` 与 `/history/net` 在未传 `days` 参数时的默认查询天数
- `storage.keep_days_local`：本地状态/历史文件保留天数；兼容旧字段 `storage.keep_days`
- `storage.dir_size_alert_mb`：数据目录大小告警阈值，单位 MB，`0` 表示关闭；启用后会自动生成内建规则 `storage_dir_size_high`
- `storage.dir_size_check_interval`：数据目录大小采样间隔，避免频繁递归扫描大目录
- `integrations.victoriametrics` / `integrations.vmagent`：可选启用本地时序存储与抓取转发，默认关闭；启用后 `mini_monitor_server` 会托管子进程，并自动生成 `vmagent` 抓取配置
- `integrations.victoriametrics.retention_days`：VictoriaMetrics 时序数据保留天数，独立于本地文件保留策略
- 随安装包分发的第三方版本固定记录在 `third_party_versions.env`，便于跟随本项目版本追踪
