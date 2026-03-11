package model

import "time"

// Snapshot 当前采集快照
type Snapshot struct {
	Timestamp time.Time     `json:"timestamp"`
	Hostname  string        `json:"hostname"`
	CPU       CPUStat       `json:"cpu"`
	Memory    MemoryStat    `json:"memory"`
	Disks     []DiskStat    `json:"disks"`
	Networks  []NetworkStat `json:"networks"`
}

type CPUStat struct {
	UsagePercent float64 `json:"usage_percent"`
}

type MemoryStat struct {
	TotalBytes  uint64  `json:"total_bytes"`
	UsedBytes   uint64  `json:"used_bytes"`
	UsedPercent float64 `json:"used_percent"`
}

type DiskStat struct {
	Mount       string  `json:"mount"`
	TotalBytes  uint64  `json:"total_bytes"`
	UsedBytes   uint64  `json:"used_bytes"`
	UsedPercent float64 `json:"used_percent"`
}

type NetworkStat struct {
	Iface   string `json:"iface"`
	RXBytes uint64 `json:"rx_bytes"`
	TXBytes uint64 `json:"tx_bytes"`
}

// MetricsAvg CPU/Memory 历史平均值
type MetricsAvg struct {
	CPU map[int]*float64 // key=分钟数, nil=数据不足
	Mem map[int]*float64
}

// AlertEvent 告警/恢复事件
type AlertEvent struct {
	Timestamp time.Time `json:"ts"`
	Rule      string    `json:"rule"`
	Status    string    `json:"status"` // "firing" or "recovered"
	Value     float64   `json:"value"`
	Severity  string    `json:"severity"`
	Message   string    `json:"message,omitempty"`
}

// RuleRuntimeState 规则运行时状态
type RuleRuntimeState struct {
	Name         string     `json:"name"`
	Status       string     `json:"status"` // "normal", "pending", "firing"
	PendingSince *time.Time `json:"pending_since,omitempty"`
	FiringSince  *time.Time `json:"firing_since,omitempty"`
	LastSentAt   *time.Time `json:"last_sent_at,omitempty"`
}

// NetworkBaseline 网络流量基线
type NetworkBaseline struct {
	RXBytes uint64 `json:"rx_bytes"`
	TXBytes uint64 `json:"tx_bytes"`
}

// ServiceState 服务持久化状态
type ServiceState struct {
	StartedAt       time.Time                  `json:"started_at"`
	NetworkBaseline map[string]NetworkBaseline  `json:"network_baseline"`
	Rules           map[string]RuleRuntimeState `json:"rules"`
	LastSnapshot    *Snapshot                   `json:"last_snapshot,omitempty"`
}
