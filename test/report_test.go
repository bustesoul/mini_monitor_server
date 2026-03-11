package test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"mini_monitor_server/internal/model"
	"mini_monitor_server/internal/report"
)

func TestTextReport(t *testing.T) {
	snap := &model.Snapshot{
		Timestamp: time.Date(2026, 3, 11, 10, 30, 0, 0, time.UTC),
		Hostname:  "s4442",
		CPU:       model.CPUStat{UsagePercent: 12.4},
		Memory:    model.MemoryStat{TotalBytes: 16 * 1024 * 1024 * 1024, UsedBytes: 10 * 1024 * 1024 * 1024, UsedPercent: 63.1},
		Disks: []model.DiskStat{
			{Mount: "/", TotalBytes: 16106127360, UsedBytes: 6335076761, UsedPercent: 42.0},
		},
		Networks: []model.NetworkStat{
			{Iface: "eth0", RXBytes: 1331439861, TXBytes: 404750221},
		},
	}

	noAvg := model.MetricsAvg{CPU: make(map[int]*float64), Mem: make(map[int]*float64)}
	text := report.TextReport(snap, nil, 7, nil, nil, noAvg)

	checks := []string{
		"Host: s4442",
		"CPU: 12.4%",
		"Memory: 63.1%",
		"Disk /: 42.0%",
		"eth0 RX:",
		"eth0 TX:",
		"Active alerts",
		"none",
	}
	for _, c := range checks {
		if !strings.Contains(text, c) {
			t.Errorf("TextReport missing %q\ngot:\n%s", c, text)
		}
	}
}

func TestTextReportWithAlerts(t *testing.T) {
	snap := &model.Snapshot{
		Timestamp: time.Now(),
		Hostname:  "host1",
	}
	noAvg := model.MetricsAvg{CPU: make(map[int]*float64), Mem: make(map[int]*float64)}
	text := report.TextReport(snap, []string{"cpu_high", "mem_high"}, 0, nil, nil, noAvg)
	if !strings.Contains(text, "cpu_high") || !strings.Contains(text, "mem_high") {
		t.Errorf("TextReport should list firing rules, got:\n%s", text)
	}
	if strings.Contains(text, "none") {
		t.Error("TextReport should not show 'none' when alerts are active")
	}
}

func TestTextReportWithAvg(t *testing.T) {
	snap := &model.Snapshot{
		Timestamp: time.Now(),
		Hostname:  "host1",
		CPU:       model.CPUStat{UsagePercent: 5.0},
		Memory:    model.MemoryStat{UsedPercent: 40.0},
	}
	cpu1 := 4.5
	mem1 := 39.0
	avg := model.MetricsAvg{
		CPU: map[int]*float64{1: &cpu1, 15: nil},
		Mem: map[int]*float64{1: &mem1, 15: nil},
	}
	text := report.TextReport(snap, nil, 0, nil, []int{1, 15}, avg)
	if !strings.Contains(text, "1m: 4.5%") {
		t.Errorf("TextReport should contain 1m avg, got:\n%s", text)
	}
	if !strings.Contains(text, "15m: --") {
		t.Errorf("TextReport should show -- for nil avg, got:\n%s", text)
	}
}

func TestJSONReport(t *testing.T) {
	snap := &model.Snapshot{
		Timestamp: time.Date(2026, 3, 11, 10, 30, 0, 0, time.UTC),
		Hostname:  "s4442",
		CPU:       model.CPUStat{UsagePercent: 12.4},
		Memory:    model.MemoryStat{UsedPercent: 63.1},
		Disks: []model.DiskStat{
			{Mount: "/", TotalBytes: 16106127360, UsedBytes: 6335076761, UsedPercent: 42.0},
		},
		Networks: []model.NetworkStat{
			{Iface: "eth0", RXBytes: 1331439861, TXBytes: 404750221},
		},
	}

	noAvg := model.MetricsAvg{CPU: make(map[int]*float64), Mem: make(map[int]*float64)}
	data, err := report.JSONReport(snap, []string{"cpu_high"}, []int{1, 15}, noAvg)
	if err != nil {
		t.Fatalf("JSONReport() error: %v", err)
	}

	var result report.JSONReportData
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}
	if result.Host != "s4442" {
		t.Errorf("Host = %q, want %q", result.Host, "s4442")
	}
	if result.CPUPercent != 12.4 {
		t.Errorf("CPUPercent = %v, want 12.4", result.CPUPercent)
	}
	if len(result.Alerts) != 1 || result.Alerts[0] != "cpu_high" {
		t.Errorf("Alerts = %v, want [cpu_high]", result.Alerts)
	}
	if len(result.Disk) != 1 {
		t.Errorf("Disk count = %d, want 1", len(result.Disk))
	}
	if result.CPUAvg == nil {
		t.Error("CPUAvg should not be nil")
	}
	if result.MemAvg == nil {
		t.Error("MemAvg should not be nil")
	}
}

func TestJSONReportWithAvg(t *testing.T) {
	snap := &model.Snapshot{
		Timestamp: time.Now(),
		Hostname:  "host1",
		CPU:       model.CPUStat{UsagePercent: 5.0},
		Memory:    model.MemoryStat{UsedPercent: 40.0},
	}
	cpu1 := 4.5
	avg := model.MetricsAvg{
		CPU: map[int]*float64{1: &cpu1},
		Mem: map[int]*float64{1: nil},
	}
	data, err := report.JSONReport(snap, nil, []int{1}, avg)
	if err != nil {
		t.Fatal(err)
	}
	var result report.JSONReportData
	json.Unmarshal(data, &result)
	if v, ok := result.CPUAvg["1m"]; !ok || v == nil || *v != 4.5 {
		t.Errorf("cpu_avg[1m] = %v, want 4.5", result.CPUAvg["1m"])
	}
	if v, ok := result.MemAvg["1m"]; !ok || v != nil {
		t.Errorf("mem_avg[1m] should be null, got %v", v)
	}
}

func TestJSONReportEmptyArrays(t *testing.T) {
	snap := &model.Snapshot{
		Timestamp: time.Now(),
		Hostname:  "host1",
	}
	noAvg := model.MetricsAvg{CPU: make(map[int]*float64), Mem: make(map[int]*float64)}
	data, err := report.JSONReport(snap, nil, nil, noAvg)
	if err != nil {
		t.Fatal(err)
	}
	// disk, network_since_start, alerts 应该是 [] 而不是 null
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)
	for _, key := range []string{"disk", "network_since_start", "alerts"} {
		v := raw[key]
		if v == nil {
			t.Errorf("%s should not be null", key)
		}
	}
}
