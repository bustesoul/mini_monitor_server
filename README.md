# mini_monitor_server

[中文](./README.md) | [English](./README_EN.md)

面向小型服务器（VPS、自托管节点）的轻量级监控守护进程。

单二进制、资源占用低、无需数据库。

## 功能特性

- 采集系统指标：CPU、内存、磁盘、网络
- 基于可配置规则的阈值告警
- 支持 Telegram Bot 远程查询
- 提供本地 HTTP API（`/report`、`/healthz`、`/history`）
- 基于文件存储，零外部依赖

## 安装

从仓库 Releases 页面下载适合你架构的发布压缩包，然后执行：

```bash
tar -xzf mini_monitor_server-*.tar.gz
cd mini_monitor_server-*/
sudo ./install.sh install
```

## 卸载

```bash
sudo ./install.sh uninstall
```

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
