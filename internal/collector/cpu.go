package collector

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"mini_monitor_server/internal/model"
)

// CPUCollector 读取 /proc/stat 计算 CPU 使用率
type CPUCollector struct {
	sampleWindow time.Duration
}

func NewCPUCollector(sampleWindow time.Duration) *CPUCollector {
	return &CPUCollector{sampleWindow: sampleWindow}
}

func (c *CPUCollector) Name() string { return "cpu" }

func (c *CPUCollector) Collect(ctx context.Context, snap *model.Snapshot) error {
	idle1, total1, err := readCPUStat()
	if err != nil {
		return err
	}

	select {
	case <-time.After(c.sampleWindow):
	case <-ctx.Done():
		return ctx.Err()
	}

	idle2, total2, err := readCPUStat()
	if err != nil {
		return err
	}

	idleDelta := float64(idle2 - idle1)
	totalDelta := float64(total2 - total1)
	if totalDelta == 0 {
		snap.CPU.UsagePercent = 0
		return nil
	}
	snap.CPU.UsagePercent = (totalDelta - idleDelta) / totalDelta * 100
	return nil
}

func readCPUStat() (idle, total uint64, err error) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0, fmt.Errorf("open /proc/stat: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return 0, 0, fmt.Errorf("unexpected /proc/stat format")
		}
		// fields: cpu user nice system idle iowait irq softirq steal ...
		var values []uint64
		for _, f := range fields[1:] {
			v, err := strconv.ParseUint(f, 10, 64)
			if err != nil {
				return 0, 0, fmt.Errorf("parse /proc/stat field: %w", err)
			}
			values = append(values, v)
		}
		for _, v := range values {
			total += v
		}
		idle = values[3] // idle 是第4个字段
		if len(values) > 4 {
			idle += values[4] // iowait
		}
		return idle, total, nil
	}
	return 0, 0, fmt.Errorf("/proc/stat: cpu line not found")
}
