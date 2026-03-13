package test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"mini_monitor_server/internal/model"
	"mini_monitor_server/internal/storage"
)

func TestStateSaveLoad(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	// 空状态加载应返回 nil
	state, err := store.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error: %v", err)
	}
	if state != nil {
		t.Fatal("LoadState() expected nil for missing file")
	}

	// 保存再加载
	now := time.Now().Truncate(time.Second)
	original := &model.ServiceState{
		StartedAt:       now,
		NetworkBaseline: map[string]model.NetworkBaseline{"eth0": {RXBytes: 100, TXBytes: 200}},
		Rules:           map[string]model.RuleRuntimeState{"cpu_high": {Name: "cpu_high", Status: "normal"}},
	}
	if err := store.SaveState(original); err != nil {
		t.Fatalf("SaveState() error: %v", err)
	}

	loaded, err := store.LoadState()
	if err != nil {
		t.Fatalf("LoadState() error: %v", err)
	}
	if !loaded.StartedAt.Equal(original.StartedAt) {
		t.Errorf("StartedAt = %v, want %v", loaded.StartedAt, original.StartedAt)
	}
	if loaded.NetworkBaseline["eth0"].RXBytes != 100 {
		t.Errorf("NetworkBaseline[eth0].RXBytes = %d, want 100", loaded.NetworkBaseline["eth0"].RXBytes)
	}
	if loaded.Rules["cpu_high"].Status != "normal" {
		t.Errorf("Rules[cpu_high].Status = %q, want %q", loaded.Rules["cpu_high"].Status, "normal")
	}
}

func TestDiskHistoryAppendAndRead(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.New(dir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	entries := []model.DiskStat{
		{Mount: "/", TotalBytes: 1000, UsedBytes: 400, UsedPercent: 40.0},
		{Mount: "/", TotalBytes: 1000, UsedBytes: 500, UsedPercent: 50.0},
	}
	for _, e := range entries {
		if err := store.AppendDiskHistory(e, now); err != nil {
			t.Fatalf("AppendDiskHistory() error: %v", err)
		}
	}

	result, err := store.ReadDiskHistory(7)
	if err != nil {
		t.Fatalf("ReadDiskHistory() error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("ReadDiskHistory() got %d entries, want 2", len(result))
	}
	if result[0].UsedPercent != 40.0 {
		t.Errorf("entry[0].UsedPercent = %v, want 40.0", result[0].UsedPercent)
	}
	if result[1].UsedPercent != 50.0 {
		t.Errorf("entry[1].UsedPercent = %v, want 50.0", result[1].UsedPercent)
	}
}

func TestNetHistoryAppendAndRead(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.New(dir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	if err := store.AppendNetHistory(model.NetworkStat{Iface: "eth0", RXBytes: 1024, TXBytes: 512}, now); err != nil {
		t.Fatal(err)
	}

	result, err := store.ReadNetHistory(7)
	if err != nil {
		t.Fatal(err)
	}
	if len(result) != 1 {
		t.Fatalf("got %d entries, want 1", len(result))
	}
	if result[0].Iface != "eth0" {
		t.Errorf("Iface = %q, want %q", result[0].Iface, "eth0")
	}
}

func TestAlertAppendAndRead(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.New(dir)
	if err != nil {
		t.Fatal(err)
	}

	now := time.Now()
	for i := 0; i < 5; i++ {
		evt := &model.AlertEvent{
			Timestamp: now.Add(time.Duration(i) * time.Minute),
			Rule:      "test_rule",
			Status:    "firing",
			Value:     float64(90 + i),
			Severity:  "warning",
		}
		if err := store.AppendAlert(evt); err != nil {
			t.Fatal(err)
		}
	}

	// 读取全部
	all, err := store.ReadAlerts(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 5 {
		t.Fatalf("ReadAlerts(0) got %d, want 5", len(all))
	}

	// 读取最后 3 条
	limited, err := store.ReadAlerts(3)
	if err != nil {
		t.Fatal(err)
	}
	if len(limited) != 3 {
		t.Fatalf("ReadAlerts(3) got %d, want 3", len(limited))
	}
	if limited[0].Value != 92 {
		t.Errorf("limited[0].Value = %v, want 92", limited[0].Value)
	}
}

func TestCleanHistory(t *testing.T) {
	dir := t.TempDir()
	store, err := storage.New(dir)
	if err != nil {
		t.Fatal(err)
	}

	old := time.Now().AddDate(0, 0, -100)
	recent := time.Now()

	// 写一条旧数据 + 一条新数据
	store.AppendDiskHistory(model.DiskStat{Mount: "/", UsedPercent: 10}, old)
	store.AppendDiskHistory(model.DiskStat{Mount: "/", UsedPercent: 20}, recent)

	store.CleanHistory(30)

	result, _ := store.ReadDiskHistory(365)
	if len(result) != 1 {
		t.Fatalf("after clean got %d entries, want 1", len(result))
	}
	if result[0].UsedPercent != 20 {
		t.Errorf("remaining entry UsedPercent = %v, want 20", result[0].UsedPercent)
	}
}

func TestStateJSON(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	state := &model.ServiceState{
		StartedAt:       now,
		NetworkBaseline: map[string]model.NetworkBaseline{"lo": {RXBytes: 0, TXBytes: 0}},
		Rules:           make(map[string]model.RuleRuntimeState),
	}

	data, err := json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}

	var loaded model.ServiceState
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatal(err)
	}
	if !loaded.StartedAt.Equal(now) {
		t.Errorf("JSON round-trip StartedAt mismatch")
	}
}

func TestDirSizeBytes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("1234"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(dir, "nested"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nested", "b.txt"), []byte("123456"), 0644); err != nil {
		t.Fatal(err)
	}

	size, err := storage.DirSizeBytes(dir)
	if err != nil {
		t.Fatal(err)
	}
	if size != 10 {
		t.Fatalf("DirSizeBytes = %d, want 10", size)
	}
}
