package metrics

import (
	"strconv"
	"strings"
	"time"

	"mini_monitor_server/internal/model"
	"mini_monitor_server/internal/storage"
)

var DefaultAvgWindows = []int{1, 15, 60}

// ParseWindows 解析逗号分隔的分钟窗口，保留输入顺序并去重。
func ParseWindows(raw string, defaultWindows []int) []int {
	var windows []int
	seen := make(map[int]struct{})
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		value, err := strconv.Atoi(part)
		if err != nil || value <= 0 {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		windows = append(windows, value)
	}
	if len(windows) > 0 {
		return windows
	}
	if len(defaultWindows) == 0 {
		return nil
	}
	cloned := make([]int, len(defaultWindows))
	copy(cloned, defaultWindows)
	return cloned
}

// ComputeAverages 计算“最近 N 个已完成分钟桶”的平均值。
// 当前正在进行中的分钟不会纳入统计，以保证 CLI / HTTP / TG 行为一致。
func ComputeAverages(store *storage.Storage, windows []int, now time.Time) model.MetricsAvg {
	avg := model.MetricsAvg{
		CPU: make(map[int]*float64, len(windows)),
		Mem: make(map[int]*float64, len(windows)),
	}
	if len(windows) == 0 {
		return avg
	}

	bucketEnd := now.UTC().Truncate(time.Minute)
	maxWindow := 0
	for _, window := range windows {
		if window > maxWindow {
			maxWindow = window
		}
	}
	if maxWindow <= 0 {
		return avg
	}

	entries, err := store.ReadMetricsHistoryRange(bucketEnd.Add(-time.Duration(maxWindow)*time.Minute), bucketEnd)
	if err != nil || len(entries) == 0 {
		return avg
	}

	for _, window := range windows {
		windowStart := bucketEnd.Add(-time.Duration(window) * time.Minute)
		var cpuSum, memSum float64
		var count int
		for _, entry := range entries {
			if entry.Timestamp.Before(windowStart) || !entry.Timestamp.Before(bucketEnd) {
				continue
			}
			cpuSum += entry.CPU
			memSum += entry.Mem
			count++
		}
		if count == 0 {
			continue
		}
		cpuAvg := cpuSum / float64(count)
		memAvg := memSum / float64(count)
		avg.CPU[window] = &cpuAvg
		avg.Mem[window] = &memAvg
	}

	return avg
}
