package daemon

import (
	"testing"
	"time"

	"mini_monitor_server/internal/storage"
)

func TestMetricsAccumulatorUsesSnapshotTimestampBucket(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.New(dir)
	if err != nil {
		t.Fatal(err)
	}

	accum := newMetricsAccumulator(store)
	first := time.Date(2026, 3, 11, 10, 0, 59, 0, time.UTC)
	second := time.Date(2026, 3, 11, 10, 1, 1, 0, time.UTC)

	accum.Record(first, 10, 20)
	accum.Record(second, 30, 40)
	accum.Flush()

	entries, err := store.ReadMetricsHistoryRange(
		time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC),
		time.Date(2026, 3, 11, 10, 2, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2", len(entries))
	}
	if !entries[0].Timestamp.Equal(time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC)) {
		t.Fatalf("first timestamp = %v, want 2026-03-11 10:00:00 UTC", entries[0].Timestamp)
	}
	if !entries[1].Timestamp.Equal(time.Date(2026, 3, 11, 10, 1, 0, 0, time.UTC)) {
		t.Fatalf("second timestamp = %v, want 2026-03-11 10:01:00 UTC", entries[1].Timestamp)
	}
	if entries[0].CPU != 10 || entries[0].Mem != 20 {
		t.Fatalf("first bucket = %+v, want cpu=10 mem=20", entries[0])
	}
	if entries[1].CPU != 30 || entries[1].Mem != 40 {
		t.Fatalf("second bucket = %+v, want cpu=30 mem=40", entries[1])
	}
}
