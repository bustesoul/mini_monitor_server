package collector

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"mini_monitor_server/internal/model"
)

// NetworkCollector 读取 /proc/net/dev 计算网络流量
type NetworkCollector struct {
	interfaces []string
	mu         sync.Mutex
	baseline   map[string]model.NetworkBaseline
}

func NewNetworkCollector(interfaces []string) *NetworkCollector {
	return &NetworkCollector{
		interfaces: interfaces,
		baseline:   make(map[string]model.NetworkBaseline),
	}
}

func (c *NetworkCollector) Name() string { return "network" }

// SetBaseline 设置网络流量基线
func (c *NetworkCollector) SetBaseline(baseline map[string]model.NetworkBaseline) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.baseline = baseline
}

// GetBaseline 获取当前基线
func (c *NetworkCollector) GetBaseline() map[string]model.NetworkBaseline {
	c.mu.Lock()
	defer c.mu.Unlock()
	result := make(map[string]model.NetworkBaseline, len(c.baseline))
	for k, v := range c.baseline {
		result[k] = v
	}
	return result
}

// InitBaseline 从当前 /proc/net/dev 读取初始化基线
func (c *NetworkCollector) InitBaseline() error {
	stats, err := readNetDev(c.interfaces)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, s := range stats {
		c.baseline[s.Iface] = model.NetworkBaseline{
			RXBytes: s.RXBytes,
			TXBytes: s.TXBytes,
		}
	}
	return nil
}

func (c *NetworkCollector) Collect(_ context.Context, snap *model.Snapshot) error {
	stats, err := readNetDev(c.interfaces)
	if err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	snap.Networks = make([]model.NetworkStat, 0, len(stats))
	for _, s := range stats {
		base := c.baseline[s.Iface]
		rxBytes := uint64(0)
		txBytes := uint64(0)
		if s.RXBytes < base.RXBytes || s.TXBytes < base.TXBytes {
			c.baseline[s.Iface] = model.NetworkBaseline{
				RXBytes: s.RXBytes,
				TXBytes: s.TXBytes,
			}
		} else {
			rxBytes = s.RXBytes - base.RXBytes
			txBytes = s.TXBytes - base.TXBytes
		}
		snap.Networks = append(snap.Networks, model.NetworkStat{
			Iface:   s.Iface,
			RXBytes: rxBytes,
			TXBytes: txBytes,
		})
	}
	return nil
}

type rawNetStat struct {
	Iface   string
	RXBytes uint64
	TXBytes uint64
}

func readNetDev(interfaces []string) ([]rawNetStat, error) {
	f, err := os.Open("/proc/net/dev")
	if err != nil {
		return nil, fmt.Errorf("open /proc/net/dev: %w", err)
	}
	defer f.Close()

	wanted := make(map[string]bool, len(interfaces))
	for _, iface := range interfaces {
		wanted[iface] = true
	}

	var stats []rawNetStat
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		iface := strings.TrimSpace(parts[0])
		if !wanted[iface] {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 10 {
			continue
		}
		rx, err := strconv.ParseUint(fields[0], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse rx bytes: %w", err)
		}
		tx, err := strconv.ParseUint(fields[8], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse tx bytes: %w", err)
		}
		stats = append(stats, rawNetStat{Iface: iface, RXBytes: rx, TXBytes: tx})
	}
	return stats, scanner.Err()
}
