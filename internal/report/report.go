package report

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"mini_monitor_server/internal/model"
	"mini_monitor_server/internal/storage"
)

// TextReport 生成文本格式报告
func TextReport(snap *model.Snapshot, firingRules []string, historyDays int, store *storage.Storage, windows []int, avg model.MetricsAvg) string {
	var b strings.Builder

	fmt.Fprintf(&b, "Host: %s\n", snap.Hostname)
	fmt.Fprintf(&b, "Time: %s\n\n", snap.Timestamp.UTC().Format("2006-01-02 15:04:05 UTC"))

	b.WriteString("System\n")
	fmt.Fprintf(&b, "  CPU: %.1f%%%s\n", snap.CPU.UsagePercent, formatAvgSuffix(windows, avg.CPU))
	fmt.Fprintf(&b, "  Memory: %.1f%%%s\n", snap.Memory.UsedPercent, formatAvgSuffix(windows, avg.Mem))
	for _, d := range snap.Disks {
		fmt.Fprintf(&b, "  Disk %s: %.1f%% (%s / %s)\n",
			d.Mount, d.UsedPercent,
			humanBytes(d.UsedBytes), humanBytes(d.TotalBytes))
	}

	if len(snap.Networks) > 0 {
		b.WriteString("\nNetwork since start\n")
		for _, n := range snap.Networks {
			fmt.Fprintf(&b, "  %s RX: %s\n", n.Iface, humanBytes(n.RXBytes))
			fmt.Fprintf(&b, "  %s TX: %s\n", n.Iface, humanBytes(n.TXBytes))
		}
	}

	b.WriteString("\nActive alerts\n")
	if len(firingRules) == 0 {
		b.WriteString("  none\n")
	} else {
		for _, name := range firingRules {
			fmt.Fprintf(&b, "  - %s\n", name)
		}
	}

	return b.String()
}

func formatAvgSuffix(windows []int, vals map[int]*float64) string {
	if len(windows) == 0 || len(vals) == 0 {
		return ""
	}
	sorted := make([]int, len(windows))
	copy(sorted, windows)
	sort.Ints(sorted)

	var parts []string
	for _, w := range sorted {
		if v, ok := vals[w]; ok && v != nil {
			parts = append(parts, fmt.Sprintf("%dm: %.1f%%", w, *v))
		} else {
			parts = append(parts, fmt.Sprintf("%dm: --", w))
		}
	}
	return " [" + strings.Join(parts, ", ") + "]"
}

// JSONReport 生成 JSON 格式报告
type JSONReportData struct {
	Host              string              `json:"host"`
	Time              string              `json:"time"`
	CPUPercent        float64             `json:"cpu_percent"`
	MemoryPercent     float64             `json:"memory_percent"`
	CPUAvg            map[string]*float64 `json:"cpu_avg"`
	MemAvg            map[string]*float64 `json:"mem_avg"`
	Disk              []model.DiskStat    `json:"disk"`
	NetworkSinceStart []model.NetworkStat `json:"network_since_start"`
	Alerts            []string            `json:"alerts"`
}

func JSONReport(snap *model.Snapshot, firingRules []string, windows []int, avg model.MetricsAvg) ([]byte, error) {
	data := JSONReportData{
		Host:              snap.Hostname,
		Time:              snap.Timestamp.UTC().Format(time.RFC3339),
		CPUPercent:        snap.CPU.UsagePercent,
		MemoryPercent:     snap.Memory.UsedPercent,
		CPUAvg:            buildAvgMap(windows, avg.CPU),
		MemAvg:            buildAvgMap(windows, avg.Mem),
		Disk:              snap.Disks,
		NetworkSinceStart: snap.Networks,
		Alerts:            firingRules,
	}
	if data.Alerts == nil {
		data.Alerts = []string{}
	}
	if data.Disk == nil {
		data.Disk = []model.DiskStat{}
	}
	if data.NetworkSinceStart == nil {
		data.NetworkSinceStart = []model.NetworkStat{}
	}
	return json.MarshalIndent(data, "", "  ")
}

func buildAvgMap(windows []int, vals map[int]*float64) map[string]*float64 {
	m := make(map[string]*float64, len(windows))
	for _, w := range windows {
		key := fmt.Sprintf("%dm", w)
		m[key] = vals[w] // nil if absent
	}
	return m
}

func humanBytes(b uint64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)
	switch {
	case b >= TB:
		return fmt.Sprintf("%.2f TB", float64(b)/float64(TB))
	case b >= GB:
		return fmt.Sprintf("%.2f GB", float64(b)/float64(GB))
	case b >= MB:
		return fmt.Sprintf("%.0f MB", float64(b)/float64(MB))
	case b >= KB:
		return fmt.Sprintf("%.0f KB", float64(b)/float64(KB))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
