package collector

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"

	"mini_monitor_server/internal/model"
)

// MemoryCollector 读取 /proc/meminfo 计算内存使用率
type MemoryCollector struct{}

func NewMemoryCollector() *MemoryCollector { return &MemoryCollector{} }

func (c *MemoryCollector) Name() string { return "memory" }

func (c *MemoryCollector) Collect(_ context.Context, snap *model.Snapshot) error {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return fmt.Errorf("open /proc/meminfo: %w", err)
	}
	defer f.Close()

	var memTotal, memAvailable uint64
	found := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			memTotal, err = parseMemInfoValue(line)
			if err != nil {
				return err
			}
			found++
		} else if strings.HasPrefix(line, "MemAvailable:") {
			memAvailable, err = parseMemInfoValue(line)
			if err != nil {
				return err
			}
			found++
		}
		if found == 2 {
			break
		}
	}
	if found < 2 {
		return fmt.Errorf("/proc/meminfo: missing MemTotal or MemAvailable")
	}

	snap.Memory.TotalBytes = memTotal
	snap.Memory.UsedBytes = memTotal - memAvailable
	if memTotal > 0 {
		snap.Memory.UsedPercent = float64(snap.Memory.UsedBytes) / float64(memTotal) * 100
	}
	return nil
}

// parseMemInfoValue 解析 "MemTotal:       16384000 kB" 格式
func parseMemInfoValue(line string) (uint64, error) {
	parts := strings.Fields(line)
	if len(parts) < 2 {
		return 0, fmt.Errorf("unexpected meminfo line: %s", line)
	}
	kb, err := strconv.ParseUint(parts[1], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("parse meminfo value: %w", err)
	}
	return kb * 1024, nil // kB → bytes
}
