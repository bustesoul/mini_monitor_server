package test

import (
	"testing"
	"time"

	"mini_monitor_server/internal/metrics"
	"mini_monitor_server/internal/storage"
)

func TestComputeAveragesUsesCompletedMinuteBuckets(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.New(dir)
	if err != nil {
		t.Fatal(err)
	}

	entries := []struct {
		ts  time.Time
		cpu float64
		mem float64
	}{
		{time.Date(2026, 3, 11, 10, 1, 0, 0, time.UTC), 1, 11},
		{time.Date(2026, 3, 11, 10, 2, 0, 0, time.UTC), 2, 12},
		{time.Date(2026, 3, 11, 10, 3, 0, 0, time.UTC), 3, 13},
		{time.Date(2026, 3, 11, 10, 4, 0, 0, time.UTC), 99, 199},
	}
	for _, entry := range entries {
		if err := store.AppendMetricsHistory(entry.cpu, entry.mem, entry.ts); err != nil {
			t.Fatal(err)
		}
	}

	now := time.Date(2026, 3, 11, 10, 4, 5, 0, time.UTC)
	avg := metrics.ComputeAverages(store, []int{1, 2, 3}, now)

	assertAvgValue(t, avg.CPU, 1, 3)
	assertAvgValue(t, avg.Mem, 1, 13)
	assertAvgValue(t, avg.CPU, 2, 2.5)
	assertAvgValue(t, avg.Mem, 2, 12.5)
	assertAvgValue(t, avg.CPU, 3, 2)
	assertAvgValue(t, avg.Mem, 3, 12)
}

func assertAvgValue(t *testing.T, values map[int]*float64, window int, want float64) {
	t.Helper()
	v, ok := values[window]
	if !ok || v == nil {
		t.Fatalf("window %d missing, want %v", window, want)
	}
	if *v != want {
		t.Fatalf("window %d = %v, want %v", window, *v, want)
	}
}
