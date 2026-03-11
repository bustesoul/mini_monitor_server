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

	text := report.TextReport(snap, nil, 7, nil)

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
	text := report.TextReport(snap, []string{"cpu_high", "mem_high"}, 0, nil)
	if !strings.Contains(text, "cpu_high") || !strings.Contains(text, "mem_high") {
		t.Errorf("TextReport should list firing rules, got:\n%s", text)
	}
	if strings.Contains(text, "none") {
		t.Error("TextReport should not show 'none' when alerts are active")
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

	data, err := report.JSONReport(snap, []string{"cpu_high"})
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
}

func TestJSONReportEmptyArrays(t *testing.T) {
	snap := &model.Snapshot{
		Timestamp: time.Now(),
		Hostname:  "host1",
	}
	data, err := report.JSONReport(snap, nil)
	if err != nil {
		t.Fatal(err)
	}
	// 确保空数组是 [] 而不是 null
	if strings.Contains(string(data), "null") {
		t.Errorf("JSON should not contain null, got:\n%s", string(data))
	}
}
