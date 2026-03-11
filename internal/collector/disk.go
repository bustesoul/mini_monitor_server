package collector

import (
	"context"
	"fmt"
	"syscall"

	"mini_monitor_server/internal/model"
)

// DiskCollector 通过 statfs 获取磁盘使用情况
type DiskCollector struct {
	mounts []string
}

func NewDiskCollector(mounts []string) *DiskCollector {
	return &DiskCollector{mounts: mounts}
}

func (c *DiskCollector) Name() string { return "disk" }

func (c *DiskCollector) Collect(_ context.Context, snap *model.Snapshot) error {
	snap.Disks = make([]model.DiskStat, 0, len(c.mounts))
	for _, mount := range c.mounts {
		stat, err := statfs(mount)
		if err != nil {
			return fmt.Errorf("statfs %s: %w", mount, err)
		}
		snap.Disks = append(snap.Disks, stat)
	}
	return nil
}

func statfs(mount string) (model.DiskStat, error) {
	var fs syscall.Statfs_t
	if err := syscall.Statfs(mount, &fs); err != nil {
		return model.DiskStat{}, err
	}

	total := fs.Blocks * uint64(fs.Bsize)
	free := fs.Bfree * uint64(fs.Bsize)
	used := total - free
	var usedPercent float64
	if total > 0 {
		usedPercent = float64(used) / float64(total) * 100
	}

	return model.DiskStat{
		Mount:       mount,
		TotalBytes:  total,
		UsedBytes:   used,
		UsedPercent: usedPercent,
	}, nil
}
