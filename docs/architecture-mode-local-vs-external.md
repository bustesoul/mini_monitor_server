# mini_monitor_server 架构设计：`mode=local` 与 `mode=external`

## 1. 文档目的

本文档定义 `mini_monitor_server` 的两种运行架构模式：

- `mode=local`
- `mode=external`

目标是明确：

- 各模式下 `mini_monitor_server` 的职责边界
- 全部已有功能在两种模式下的去向
- `VictoriaMetrics plugin` 的职责范围
- 本地存储、外部时序存储、告警、Bot、HTTP API、CLI 的分工
- 后续实现与迁移顺序

本文档是架构设计文档，不是实现说明书，也不是用户安装手册。

## 2. 背景与问题

当前项目已经具备以下能力：

- 系统指标采集：CPU、内存、磁盘、网络
- 本地状态与历史存储
- 阈值规则告警
- HTTP API
- Telegram Bot
- `/metrics` 暴露
- 可选托管 `vmagent` 与 `VictoriaMetrics single-node`

现状的问题在于：

- 本地存储与外部时序存储职责还没有完全分清
- `basic/full` 更像安装模板，不足以表达运行模式
- 当引入 VictoriaMetrics 后，哪些能力仍应保留在本地，哪些应迁移到外部后端，需要统一定义

因此，系统需要从“安装套餐思维”转向“运行模式思维”。

## 3. 设计目标

### 3.1 核心目标

引入统一运行模式：

- `mode=local`
  `mini_monitor_server` 自己负责采集、存储、统计、报告、告警、Bot。
- `mode=external`
  `mini_monitor_server` 只负责采集、当前状态、告警、基础 Bot、标准指标暴露。
  历史存储、统计、趋势查询、报表聚合通过 `VictoriaMetrics plugin` 查询外部后端完成。

### 3.2 设计原则

- KISS：不引入不必要的多后端复杂组合
- 职责清晰：本地状态、本地历史、外部时序、事件历史边界明确
- 演进兼容：允许从当前实现平滑迁移
- 降级可控：external 模式下外部查询失败时，基础能力仍可工作

## 4. 非目标

本阶段不做以下事情：

- 不把告警引擎迁移到 `VictoriaMetrics` / `vmalert`
- 不把指标写入逻辑塞进 `VictoriaMetrics plugin`
- 不把 `vmagent` 写入链路改成应用内 push
- 不把所有事件历史统一抽象成外部事件存储
- 不追求 external 模式下“完全无本地状态”

## 5. 总体架构

### 5.1 `mode=local`

职责定义：

- 采集：本地完成
- 存储：本地完成
- 统计：本地完成
- 报告：本地完成
- 告警：本地完成
- Bot：本地完成

数据流：

1. 采集器生成当前快照
2. 规则引擎评估当前快照
3. 本地保存 state 与历史数据
4. 本地生成 avg / history / report
5. notifier 发送告警
6. HTTP / CLI / Telegram 直接读取本地能力

### 5.2 `mode=external`

职责定义：

- 采集：本地完成
- 当前状态：本地完成
- 告警：本地完成
- 基础 Bot：本地完成
- 标准指标暴露：本地完成
- 历史存储：外部完成
- 统计计算：外部完成
- 趋势查询：外部完成
- 报表聚合：外部完成

数据流：

1. 采集器生成当前快照
2. 规则引擎评估当前快照
3. 仅保存最小运行状态
4. `/metrics` 暴露给 `vmagent`
5. `vmagent` 写入 `VictoriaMetrics`
6. HTTP / CLI / Telegram 中需要 avg / history / 聚合的能力通过 plugin 查询 `VictoriaMetrics`
7. notifier 发送告警

### 5.3 `VictoriaMetrics plugin`

plugin 是“查询适配层”，不是“写入组件”，也不是“第二个 daemon”。

它只负责：

- avg 查询
- range 查询
- 报表聚合

它不负责：

- 指标写入
- 告警判断
- 通知发送
- 最小 state 持久化
- 事件历史存储
- 托管 `vmagent`
- 托管 `VictoriaMetrics`

## 6. 全部已有功能去向

本节要求完整覆盖当前已有功能，不能遗漏。

### 6.1 CLI 功能

#### `mini_monitor_server daemon`

当前职责：

- 启动守护进程
- 初始化状态
- 采集
- 评估规则
- 启动 HTTP server
- 启动 Telegram Bot
- 可选托管 `vmagent` / `VictoriaMetrics`

去向：

- `local`：保留
- `external`：保留

说明：

- `daemon` 是两种模式的统一入口，不拆分。

#### `mini_monitor_server report`

当前职责：

- 输出当前系统报告
- 可带 `--json`
- 可带 `--avg`
- 当前 avg 依赖本地 `metrics_history.ndjson`

去向：

- `local`：保留，avg 继续走本地 store
- `external`：保留，但 avg / 聚合数据改由 plugin 查询 `VictoriaMetrics`

降级策略建议：

- 当前快照仍然输出
- plugin 查询失败时 avg 显示 `--`

#### `mini_monitor_server check`

当前职责：

- 校验配置文件

去向：

- `local`：保留
- `external`：保留

新增要求：

- `external` 模式下需要额外校验 external backend 配置是否完整

#### `mini_monitor_server version`

当前职责：

- 输出版本

去向：

- `local`：保留
- `external`：保留

### 6.2 HTTP API 功能

#### `/healthz`

当前职责：

- 返回健康状态

去向：

- `local`：保留
- `external`：保留

#### `/metrics`

当前职责：

- 暴露 Prometheus 文本格式指标
- 支撑 `vmagent` 抓取

去向：

- `local`：保留
- `external`：保留，且是核心能力

#### `/report`

当前职责：

- 返回当前系统报告
- 支持 text/json
- 支持 avg 窗口参数

去向：

- `local`：保留，本地生成 avg / 报告
- `external`：保留，当前快照本地，avg / 统计字段通过 plugin 获取

#### `/history/disk`

当前职责：

- 读取本地 `disk_history.ndjson`

去向：

- `local`：保留，本地历史接口
- `external`：迁移到 plugin range 查询，不再读本地 `disk_history.ndjson`

#### `/history/net`

当前职责：

- 读取本地 `net_history.ndjson`

去向：

- `local`：保留，本地历史接口
- `external`：迁移到 plugin range 查询，不再读本地 `net_history.ndjson`

#### `/alerts`

当前职责：

- 读取本地 `alerts.ndjson`
- 返回最近告警事件

去向：

- `local`：保留，本地历史事件接口
- `external`：`v1` 明确不提供历史事件查询

原因：

- `VictoriaMetrics plugin` 不负责事件历史存储
- external 模式定位为“外部时序统计”，不是“外部事件存储”

### 6.3 Telegram Bot 功能

#### `/help`

当前职责：

- 展示可用命令

去向：

- `local`：保留
- `external`：保留

新增要求：

- 应基于当前模式动态展示命令或说明降级

#### `/report`

当前职责：

- 返回完整文本报告
- 支持 avg 参数

去向：

- `local`：保留，本地 avg
- `external`：保留，avg / 聚合走 plugin

#### `/cpu`

当前职责：

- 显示 CPU 当前值
- 可附带 avg 窗口

去向：

- `local`：保留，本地 avg
- `external`：保留，avg 走 plugin

#### `/mem`

当前职责：

- 显示内存当前值
- 可附带 avg 窗口

去向：

- `local`：保留，本地 avg
- `external`：保留，avg 走 plugin

#### `/disk`

当前职责：

- 显示当前磁盘使用情况

去向：

- `local`：保留
- `external`：保留

原因：

- 只依赖当前 snapshot

#### `/net`

当前职责：

- 显示当前网络流量

去向：

- `local`：保留
- `external`：保留

原因：

- 只依赖当前 snapshot

#### `/alerts`

当前职责：

- 显示最近告警事件列表
- 依赖本地 `alerts.ndjson`

去向：

- `local`：保留
- `external`：`v1` 不提供历史事件列表

说明：

- external 模式下不要伪装成“最近 firing 规则”，避免语义混乱
- 应明确返回“不支持历史事件查询”

### 6.4 采集器功能

#### CPU 采集器

去向：

- `local`：保留
- `external`：保留

#### Memory 采集器

去向：

- `local`：保留
- `external`：保留

#### Disk 采集器

去向：

- `local`：保留
- `external`：保留

#### Network 采集器

去向：

- `local`：保留
- `external`：保留

说明：

- external 模式仍需要当前 snapshot 和 `/metrics`

### 6.5 本地存储功能

#### `state.json`

当前职责：

- 保存 `LastSnapshot`
- 保存规则状态
- 保存网络 baseline
- 支撑重启恢复

去向：

- `local`：保留
- `external`：保留

说明：

- external 模式下只保留最小运行态，不是完全无本地状态

#### `metrics_history.ndjson`

当前职责：

- 保存 CPU / 内存分钟历史
- 供本地 avg 计算使用

去向：

- `local`：保留
- `external`：移除本地写入

替代：

- avg 改由 plugin 查询 `VictoriaMetrics`

#### `disk_history.ndjson`

当前职责：

- 保存磁盘历史

去向：

- `local`：保留
- `external`：移除本地写入

替代：

- range 查询走 plugin

#### `net_history.ndjson`

当前职责：

- 保存网络历史

去向：

- `local`：保留
- `external`：移除本地写入

替代：

- range 查询走 plugin

#### `alerts.ndjson`

当前职责：

- 保存告警事件历史

去向：

- `local`：保留
- `external`：移除本地写入

说明：

- external `v1` 不提供历史事件查询

### 6.6 统计与报告功能

#### 本地 avg 计算

当前职责：

- 通过 `metrics.ComputeAverages` 读取本地分钟历史

去向：

- `local`：保留
- `external`：替换为 plugin avg 查询

#### 本地 report 拼装

当前职责：

- 文本报告
- JSON 报告

去向：

- `local`：保留
- `external`：保留最终拼装逻辑

变化：

- 报告所需 avg / 聚合字段由 plugin 提供

### 6.7 告警功能

#### 规则引擎

当前职责：

- 规则状态机
- firing / recovered 事件

去向：

- `local`：保留
- `external`：保留

说明：

- external 模式仍由本地规则引擎做实时告警判断

#### dedup / repeat / recovery

当前职责：

- 去重
- 重复提醒
- 恢复通知

去向：

- `local`：保留
- `external`：保留

说明：

- 依赖最小 state 本地持久化

#### notifier

当前职责：

- log notifier
- telegram notifier

去向：

- `local`：保留
- `external`：保留

### 6.8 集成功能

#### integration manager

当前职责：

- 可选托管 `vmagent`
- 可选托管 `VictoriaMetrics`

去向：

- `local`：可选保留
- `external`：推荐保留

说明：

- 这是部署能力，不是 mode 本身

#### bundled third-party binaries

当前职责：

- 打包固定版本 `VictoriaMetrics` / `vmagent`
- install script 一起安装

去向：

- `local`：保留
- `external`：保留

说明：

- 与 mode 无关，属于安装交付能力

## 7. 模式下的本地持久化边界

### 7.1 `local`

本地持久化内容：

- `state.json`
- `metrics_history.ndjson`
- `disk_history.ndjson`
- `net_history.ndjson`
- `alerts.ndjson`

### 7.2 `external`

本地持久化内容：

- `state.json`
- 运行时生成配置文件，例如 integration 相关文件

不再本地持久化：

- `metrics_history.ndjson`
- `disk_history.ndjson`
- `net_history.ndjson`
- `alerts.ndjson`

明确说明：

- `external` 不是“完全无本地状态”
- `external` 是“无本地历史时序与本地统计”

## 8. `VictoriaMetrics plugin` 设计职责

### 8.1 输入

- host 标识
- 时间范围
- 聚合窗口
- 查询目标指标
- 当前 snapshot 需要补齐的统计字段

### 8.2 输出

- CPU avg
- Memory avg
- Disk / Network / CPU / Memory 的 range 结果
- 报表所需聚合字段

### 8.3 对外接口建议

建议抽象三类接口：

- `AvgProvider`
- `HistoryProvider`
- `ReportProvider`

`local` 模式使用本地实现  
`external` 模式使用 `VictoriaMetrics plugin` 实现

### 8.4 不负责的内容

- 写入 VictoriaMetrics
- 启停 VictoriaMetrics
- 启停 vmagent
- 生成 `/metrics`
- 告警规则评估
- 事件历史管理

## 9. external 模式下的降级策略

### 9.1 仍然必须可用

- 当前快照采集
- `/metrics`
- 规则判断
- notifier
- `/healthz`
- Telegram 基础命令：`/help`、`/disk`、`/net`

### 9.2 依赖 plugin，可降级

- `/report`
- CLI `report`
- `/cpu` 的 avg
- `/mem` 的 avg
- `/history/disk`
- `/history/net`
- Telegram `/report`

### 9.3 明确不支持

- `/alerts` 历史事件查询
- Telegram `/alerts` 历史事件列表

### 9.4 plugin 失败时建议行为

- 当前值仍返回
- avg / 聚合字段返回 `--` 或明确 unavailable
- 历史接口返回 `503`
- Bot 返回“external backend unavailable”

## 10. 配置模型建议

建议引入统一模式字段：

```yaml
mode: "local"   # local | external
```

辅助配置：

```yaml
external_backend:
  type: "victoriametrics"
```

说明：

- 不建议用大量离散开关来表达 mode 语义
- `mode` 应是第一层架构开关
- 历史 / avg / report backend 应随 mode 统一切换

## 11. 与安装方式的关系

安装方式只是默认配置模板，不应承担架构语义本身。

建议映射关系：

- `install-basic` -> 生成 `mode=local` 默认配置
- `install-full` -> 生成 `mode=external` 默认配置，并启用 `vmagent + VictoriaMetrics`

因此：

- `basic/full` 是安装模板
- `local/external` 是运行模式

## 12. 风险与取舍

### 12.1 已明确保留

- external 模式保留当前快照
- external 模式保留规则告警
- external 模式保留最小 state
- external 模式保留基础 Bot

### 12.2 已明确迁移

- avg
- history
- report 聚合

### 12.3 已明确移除或降级

- external `v1` 不提供历史告警事件查询
- external `v1` 不本地保存 CPU/内存分钟历史
- external `v1` 不本地保存磁盘/网络历史

这些都是显式取舍，不是遗漏。

## 13. 实施顺序建议

1. 引入 `mode=local|external`
2. 抽象 `AvgProvider / HistoryProvider / ReportProvider`
3. 实现 `LocalProvider`
4. 实现 `VictoriaMetricsProvider`
5. 按 mode 裁剪本地历史落盘行为
6. 调整 HTTP / CLI / Telegram 在 external 模式下的行为与错误语义
7. 最后把安装模板正式映射到 mode 语义

## 14. 结论

`mode=local` 与 `mode=external` 的核心区别不是“是否安装了 VictoriaMetrics”，而是：

- `local`：`mini_monitor_server` 负责本地历史、统计与报告
- `external`：`mini_monitor_server` 只负责采集、当前状态、告警与基础交互，历史与统计由外部后端负责

这能把系统边界从“安装套餐”提升为“明确架构模式”，也能为后续 `VictoriaMetrics plugin` 的实现提供稳定接口基础。
