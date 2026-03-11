package command

import (
	"context"
	"fmt"
	"strings"
	"time"

	"mini_monitor_server/internal/metrics"
	"mini_monitor_server/internal/model"
	"mini_monitor_server/internal/report"
	"mini_monitor_server/internal/rule"
)

type ReportCmd struct {
	getSnapshot   func() *model.Snapshot
	getMetricsAvg func(time.Time, []int) model.MetricsAvg
	engine        *rule.Engine
}

func (c *ReportCmd) Name() string        { return "report" }
func (c *ReportCmd) Description() string { return "Show system report" }

func (c *ReportCmd) Execute(_ context.Context, args string) (string, error) {
	snap := c.getSnapshot()
	if snap == nil {
		return "No data yet.", nil
	}
	windows := metrics.ParseWindows(args, metrics.DefaultAvgWindows)
	var avg model.MetricsAvg
	if c.getMetricsAvg != nil {
		avg = c.getMetricsAvg(snap.Timestamp, windows)
	} else {
		avg = model.MetricsAvg{CPU: make(map[int]*float64), Mem: make(map[int]*float64)}
	}
	return report.TextReport(snap, c.engine.FiringRules(), 0, nil, windows, avg), nil
}

type CPUCmd struct {
	getSnapshot   func() *model.Snapshot
	getMetricsAvg func(time.Time, []int) model.MetricsAvg
}

func (c *CPUCmd) Name() string        { return "cpu" }
func (c *CPUCmd) Description() string { return "Show CPU usage" }

func (c *CPUCmd) Execute(_ context.Context, args string) (string, error) {
	snap := c.getSnapshot()
	if snap == nil {
		return "No data yet.", nil
	}
	result := fmt.Sprintf("CPU: %.1f%%", snap.CPU.UsagePercent)
	windows := metrics.ParseWindows(args, nil)
	if len(windows) > 0 && c.getMetricsAvg != nil {
		avg := c.getMetricsAvg(snap.Timestamp, windows)
		result += formatWindowValues(windows, avg.CPU)
	}
	return result, nil
}

type MemCmd struct {
	getSnapshot   func() *model.Snapshot
	getMetricsAvg func(time.Time, []int) model.MetricsAvg
}

func (c *MemCmd) Name() string        { return "mem" }
func (c *MemCmd) Description() string { return "Show memory usage" }

func (c *MemCmd) Execute(_ context.Context, args string) (string, error) {
	snap := c.getSnapshot()
	if snap == nil {
		return "No data yet.", nil
	}
	result := fmt.Sprintf("Memory: %.1f%% (%s / %s)",
		snap.Memory.UsedPercent,
		humanBytes(snap.Memory.UsedBytes),
		humanBytes(snap.Memory.TotalBytes))
	windows := metrics.ParseWindows(args, nil)
	if len(windows) > 0 && c.getMetricsAvg != nil {
		avg := c.getMetricsAvg(snap.Timestamp, windows)
		result += formatWindowValues(windows, avg.Mem)
	}
	return result, nil
}

type DiskCmd struct {
	getSnapshot func() *model.Snapshot
}

func (c *DiskCmd) Name() string        { return "disk" }
func (c *DiskCmd) Description() string { return "Show disk usage" }

func (c *DiskCmd) Execute(_ context.Context, _ string) (string, error) {
	snap := c.getSnapshot()
	if snap == nil {
		return "No data yet.", nil
	}
	result := "Disk:\n"
	for _, d := range snap.Disks {
		result += fmt.Sprintf("  %s: %.1f%% (%s / %s)\n",
			d.Mount, d.UsedPercent,
			humanBytes(d.UsedBytes), humanBytes(d.TotalBytes))
	}
	return result, nil
}

type NetCmd struct {
	getSnapshot func() *model.Snapshot
}

func (c *NetCmd) Name() string        { return "net" }
func (c *NetCmd) Description() string { return "Show network traffic" }

func (c *NetCmd) Execute(_ context.Context, _ string) (string, error) {
	snap := c.getSnapshot()
	if snap == nil {
		return "No data yet.", nil
	}
	result := "Network since start:\n"
	for _, n := range snap.Networks {
		result += fmt.Sprintf("  %s RX: %s  TX: %s\n",
			n.Iface, humanBytes(n.RXBytes), humanBytes(n.TXBytes))
	}
	return result, nil
}

func formatWindowValues(windows []int, vals map[int]*float64) string {
	var parts []string
	for _, w := range windows {
		if v, ok := vals[w]; ok && v != nil {
			parts = append(parts, fmt.Sprintf("%dm: %.1f%%", w, *v))
		} else {
			parts = append(parts, fmt.Sprintf("%dm: --", w))
		}
	}
	return " [" + strings.Join(parts, ", ") + "]"
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
