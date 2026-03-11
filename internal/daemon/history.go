package daemon

import (
	"log/slog"
	"sync"
	"time"

	"mini_monitor_server/internal/storage"
)

type metricsAccumulator struct {
	mu             sync.Mutex
	cpuSum, memSum float64
	count          int
	curMin         int64 // UTC unix timestamp / 60
	store          *storage.Storage
}

func newMetricsAccumulator(store *storage.Storage) *metricsAccumulator {
	return &metricsAccumulator{store: store}
}

// Record 累加一次采集的 CPU/Memory 值。跨分钟时 flush 上一分钟的平均值。
func (a *metricsAccumulator) Record(ts time.Time, cpu, mem float64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	bucketMin := ts.UTC().Truncate(time.Minute).Unix() / 60
	if a.count > 0 && bucketMin != a.curMin {
		a.flushLocked()
	}
	if a.count == 0 {
		a.curMin = bucketMin
	}
	a.cpuSum += cpu
	a.memSum += mem
	a.count++
}

// Flush 强制写入当前累积数据（shutdown 时调用）
func (a *metricsAccumulator) Flush() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.flushLocked()
}

func (a *metricsAccumulator) flushLocked() {
	if a.count == 0 {
		return
	}
	cpuAvg := a.cpuSum / float64(a.count)
	memAvg := a.memSum / float64(a.count)
	ts := time.Unix(a.curMin*60, 0).UTC()

	if err := a.store.AppendMetricsHistory(cpuAvg, memAvg, ts); err != nil {
		slog.Error("append metrics history failed", "error", err)
	}

	a.cpuSum = 0
	a.memSum = 0
	a.count = 0
}
