package test

import (
	"testing"
	"time"

	"mini_monitor_server/internal/storage"
)

func TestMetricsHistoryAppendAndRead(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.New(dir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	// 写入 3 条记录：-5m, -2m, -0m
	entries := []struct {
		cpu, mem float64
		offset   time.Duration
	}{
		{3.0, 40.0, -5 * time.Minute},
		{4.0, 42.0, -2 * time.Minute},
		{5.0, 44.0, 0},
	}
	for _, e := range entries {
		if err := store.AppendMetricsHistory(e.cpu, e.mem, now.Add(e.offset)); err != nil {
			t.Fatal(err)
		}
	}

	// 读最近 3 分钟 → 应返回 2 条（-2m 和 0m）
	got, err := store.ReadMetricsHistory(3)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("ReadMetricsHistory(3) = %d entries, want 2", len(got))
	}

	// 读最近 10 分钟 → 应返回 3 条
	got, err = store.ReadMetricsHistory(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("ReadMetricsHistory(10) = %d entries, want 3", len(got))
	}
}

func TestMetricsHistoryReadEmpty(t *testing.T) {
	dir := t.TempDir()
	store, _ := storage.New(dir)

	got, err := store.ReadMetricsHistory(60)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty, got %d entries", len(got))
	}
}

func TestMetricsHistoryClean(t *testing.T) {
	dir := t.TempDir()
	store, _ := storage.New(dir)

	old := time.Now().AddDate(0, 0, -10)
	recent := time.Now()

	store.AppendMetricsHistory(1.0, 20.0, old)
	store.AppendMetricsHistory(2.0, 30.0, recent)

	store.CleanHistory(7) // 清理 7 天前的

	got, _ := store.ReadMetricsHistory(60 * 24 * 30) // 读 30 天
	if len(got) != 1 {
		t.Fatalf("after clean, got %d entries, want 1", len(got))
	}
	if got[0].CPU != 2.0 {
		t.Errorf("remaining entry CPU = %v, want 2.0", got[0].CPU)
	}
}
