package test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"mini_monitor_server/internal/config"
)

func TestLoadConfig(t *testing.T) {
	yaml := `
server:
  listen: "127.0.0.1:9090"
collector:
  interval: "30s"
  cpu_sample_window: "1s"
storage:
  dir: "/tmp/test_monitor"
  keep_days: 30
history:
  default_days: 14
network:
  enabled: true
  interfaces: ["eth0"]
disk:
  enabled: true
  mounts: ["/"]
  sample_daily_at: "02:00"
rules:
  - name: "cpu_high"
    type: "cpu_used_percent"
    threshold: 90
    for: "5m"
    severity: "warning"
notify:
  telegram:
    enabled: false
`
	path := writeTempFile(t, "config.yaml", yaml)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Server.Listen != "127.0.0.1:9090" {
		t.Errorf("Server.Listen = %q, want %q", cfg.Server.Listen, "127.0.0.1:9090")
	}
	if cfg.Collector.Interval.Seconds() != 30 {
		t.Errorf("Collector.Interval = %v, want 30s", cfg.Collector.Interval)
	}
	if cfg.Storage.KeepDays != 30 {
		t.Errorf("Storage.KeepDays = %d, want 30", cfg.Storage.KeepDays)
	}
	if cfg.History.DefaultDays != 14 {
		t.Errorf("History.DefaultDays = %d, want 14", cfg.History.DefaultDays)
	}
	if len(cfg.Rules) != 1 {
		t.Fatalf("len(Rules) = %d, want 1", len(cfg.Rules))
	}
	if cfg.Rules[0].Name != "cpu_high" {
		t.Errorf("Rules[0].Name = %q, want %q", cfg.Rules[0].Name, "cpu_high")
	}
}

func TestLoadConfigDefaults(t *testing.T) {
	yaml := `
rules: []
notify:
  telegram:
    enabled: false
`
	path := writeTempFile(t, "defaults.yaml", yaml)
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if cfg.Server.Listen != "127.0.0.1:18080" {
		t.Errorf("default Server.Listen = %q, want %q", cfg.Server.Listen, "127.0.0.1:18080")
	}
	if cfg.Collector.Interval.Seconds() != 60 {
		t.Errorf("default Collector.Interval = %v, want 60s", cfg.Collector.Interval)
	}
	if cfg.Storage.KeepDays != 90 {
		t.Errorf("default Storage.KeepDays = %d, want 90", cfg.Storage.KeepDays)
	}
	if cfg.History.DefaultDays != 7 {
		t.Errorf("default History.DefaultDays = %d, want 7", cfg.History.DefaultDays)
	}
}

func TestValidateRules(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.Config
		wantErr bool
	}{
		{
			name: "valid cpu rule",
			cfg: config.Config{
				Collector: defaultCollector(),
				Rules: []config.RuleConfig{
					{Name: "cpu_high", Type: "cpu_used_percent", Threshold: 90, For: duration(5 * time.Minute), Severity: "warning"},
				},
			},
		},
		{
			name: "missing rule name",
			cfg: config.Config{
				Collector: defaultCollector(),
				Rules:     []config.RuleConfig{{Type: "cpu_used_percent", Threshold: 90, For: duration(5 * time.Minute), Severity: "warning"}},
			},
			wantErr: true,
		},
		{
			name: "unknown rule type",
			cfg: config.Config{
				Collector: defaultCollector(),
				Rules:     []config.RuleConfig{{Name: "x", Type: "unknown", Threshold: 90, For: duration(5 * time.Minute), Severity: "warning"}},
			},
			wantErr: true,
		},
		{
			name: "disk rule without mount",
			cfg: config.Config{
				Collector: defaultCollector(),
				Rules:     []config.RuleConfig{{Name: "d", Type: "disk_used_percent", Threshold: 90, For: duration(5 * time.Minute), Severity: "critical"}},
			},
			wantErr: true,
		},
		{
			name: "threshold out of range",
			cfg: config.Config{
				Collector: defaultCollector(),
				Rules:     []config.RuleConfig{{Name: "x", Type: "cpu_used_percent", Threshold: 0, For: duration(5 * time.Minute), Severity: "warning"}},
			},
			wantErr: true,
		},
		{
			name: "duplicate rule names",
			cfg: config.Config{
				Collector: defaultCollector(),
				Rules: []config.RuleConfig{
					{Name: "dup", Type: "cpu_used_percent", Threshold: 90, For: duration(5 * time.Minute), Severity: "warning"},
					{Name: "dup", Type: "memory_used_percent", Threshold: 80, For: duration(5 * time.Minute), Severity: "warning"},
				},
			},
			wantErr: true,
		},
		{
			name: "rule without for",
			cfg: config.Config{
				Collector: defaultCollector(),
				Rules: []config.RuleConfig{
					{Name: "cpu_high", Type: "cpu_used_percent", Threshold: 90, Severity: "warning"},
				},
			},
			wantErr: true,
		},
		{
			name: "telegram enabled without token",
			cfg: config.Config{
				Collector: defaultCollector(),
				Notify:    config.NotifyConfig{Telegram: config.TelegramConfig{Enabled: true}},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := config.Validate(&tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func defaultCollector() config.CollectorConfig {
	var c config.CollectorConfig
	c.Interval.Duration = 60_000_000_000       // 60s
	c.CPUSampleWindow.Duration = 1_000_000_000 // 1s
	return c
}

func duration(d time.Duration) config.Duration {
	return config.Duration{Duration: d}
}

func writeTempFile(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}
