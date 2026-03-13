package daemon

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"mini_monitor_server/internal/config"
	"mini_monitor_server/internal/model"
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

func TestHistoryDefaultDaysWiredFromConfigToHTTP(t *testing.T) {
	cfg := &config.Config{
		Server:  config.ServerConfig{Listen: "127.0.0.1:0"},
		Storage: config.StorageConfig{Dir: t.TempDir()},
		History: config.HistoryConfig{DefaultDays: 2},
	}

	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	now := time.Now()
	recentDisk := model.DiskStat{Mount: "/", UsedPercent: 40}
	oldDisk := model.DiskStat{Mount: "/old", UsedPercent: 80}
	if err := d.store.AppendDiskHistory(recentDisk, now.Add(-24*time.Hour)); err != nil {
		t.Fatalf("AppendDiskHistory(recent) error: %v", err)
	}
	if err := d.store.AppendDiskHistory(oldDisk, now.Add(-72*time.Hour)); err != nil {
		t.Fatalf("AppendDiskHistory(old) error: %v", err)
	}

	recentNet := model.NetworkStat{Iface: "eth0", RXBytes: 100, TXBytes: 200}
	oldNet := model.NetworkStat{Iface: "eth1", RXBytes: 300, TXBytes: 400}
	if err := d.store.AppendNetHistory(recentNet, now.Add(-24*time.Hour)); err != nil {
		t.Fatalf("AppendNetHistory(recent) error: %v", err)
	}
	if err := d.store.AppendNetHistory(oldNet, now.Add(-72*time.Hour)); err != nil {
		t.Fatalf("AppendNetHistory(old) error: %v", err)
	}

	srv := httptest.NewServer(d.httpSrv.Handler())
	defer srv.Close()

	diskResp, err := http.Get(srv.URL + "/history/disk")
	if err != nil {
		t.Fatalf("GET /history/disk error: %v", err)
	}
	defer diskResp.Body.Close()
	if diskResp.StatusCode != http.StatusOK {
		t.Fatalf("/history/disk status = %d, want 200", diskResp.StatusCode)
	}
	var diskEntries []storage.DiskHistoryEntry
	if err := json.NewDecoder(diskResp.Body).Decode(&diskEntries); err != nil {
		t.Fatalf("decode /history/disk error: %v", err)
	}
	if len(diskEntries) != 1 {
		t.Fatalf("/history/disk entries = %d, want 1", len(diskEntries))
	}
	if diskEntries[0].Mount != recentDisk.Mount {
		t.Fatalf("/history/disk mount = %q, want %q", diskEntries[0].Mount, recentDisk.Mount)
	}

	netResp, err := http.Get(srv.URL + "/history/net")
	if err != nil {
		t.Fatalf("GET /history/net error: %v", err)
	}
	defer netResp.Body.Close()
	if netResp.StatusCode != http.StatusOK {
		t.Fatalf("/history/net status = %d, want 200", netResp.StatusCode)
	}
	var netEntries []storage.NetHistoryEntry
	if err := json.NewDecoder(netResp.Body).Decode(&netEntries); err != nil {
		t.Fatalf("decode /history/net error: %v", err)
	}
	if len(netEntries) != 1 {
		t.Fatalf("/history/net entries = %d, want 1", len(netEntries))
	}
	if netEntries[0].Iface != recentNet.Iface {
		t.Fatalf("/history/net iface = %q, want %q", netEntries[0].Iface, recentNet.Iface)
	}
}

func TestStartupMaintenanceCleansHistory(t *testing.T) {
	cfg := &config.Config{
		Server:  config.ServerConfig{Listen: "127.0.0.1:0"},
		Storage: config.StorageConfig{Dir: t.TempDir(), KeepDaysLocal: 7},
	}

	d, err := New(cfg)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}

	old := time.Now().AddDate(0, 0, -30)
	recent := time.Now()
	if err := d.store.AppendDiskHistory(model.DiskStat{Mount: "/", UsedPercent: 10}, old); err != nil {
		t.Fatal(err)
	}
	if err := d.store.AppendDiskHistory(model.DiskStat{Mount: "/", UsedPercent: 20}, recent); err != nil {
		t.Fatal(err)
	}

	d.performStartupMaintenance()

	entries, err := d.store.ReadDiskHistory(365)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if entries[0].UsedPercent != 20 {
		t.Fatalf("remaining UsedPercent = %v, want 20", entries[0].UsedPercent)
	}
}

func TestBuildRuntimeRulesAddsStorageDirAlert(t *testing.T) {
	cfg := &config.Config{
		Collector: config.CollectorConfig{Interval: config.Duration{Duration: time.Minute}},
		Storage:   config.StorageConfig{DirSizeAlertMB: 2048},
		Rules: []config.RuleConfig{
			{Name: "cpu_high", Type: "cpu_used_percent", Threshold: 90, Severity: "warning", For: config.Duration{Duration: 5 * time.Minute}},
		},
	}

	rules := buildRuntimeRules(cfg)
	if len(rules) != 2 {
		t.Fatalf("len(rules) = %d, want 2", len(rules))
	}
	last := rules[len(rules)-1]
	if last.Name != config.StorageDirAlertRuleName {
		t.Fatalf("last rule name = %q, want %q", last.Name, config.StorageDirAlertRuleName)
	}
	if last.Type != "storage_dir_size_mb" || last.Threshold != 2048 {
		t.Fatalf("unexpected storage rule: %+v", last)
	}
}
