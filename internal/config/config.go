package config

import (
	"fmt"
	"net"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

const StorageDirAlertRuleName = "storage_dir_size_high"

type Config struct {
	Server       ServerConfig       `yaml:"server"`
	Collector    CollectorConfig    `yaml:"collector"`
	Storage      StorageConfig      `yaml:"storage"`
	History      HistoryConfig      `yaml:"history"`
	Network      NetworkConfig      `yaml:"network"`
	Disk         DiskConfig         `yaml:"disk"`
	Alerts       AlertsConfig       `yaml:"alerts"`
	Rules        []RuleConfig       `yaml:"rules"`
	Notify       NotifyConfig       `yaml:"notify"`
	Integrations IntegrationsConfig `yaml:"integrations"`
}

type ServerConfig struct {
	Listen string `yaml:"listen"`
}

type CollectorConfig struct {
	Interval        Duration `yaml:"interval"`
	CPUSampleWindow Duration `yaml:"cpu_sample_window"`
}

type StorageConfig struct {
	Dir                  string   `yaml:"dir"`
	KeepDaysLocal        int      `yaml:"keep_days_local"`
	KeepDaysLegacy       int      `yaml:"keep_days"`
	DirSizeAlertMB       uint64   `yaml:"dir_size_alert_mb"`
	DirSizeCheckInterval Duration `yaml:"dir_size_check_interval"`
}

type NetworkConfig struct {
	Enabled              bool     `yaml:"enabled"`
	Interfaces           []string `yaml:"interfaces"`
	ResetBaselineOnStart bool     `yaml:"reset_baseline_on_start"`
	DailyRollup          bool     `yaml:"daily_rollup"`
}

type DiskConfig struct {
	Enabled       bool     `yaml:"enabled"`
	Mounts        []string `yaml:"mounts"`
	SampleDailyAt string   `yaml:"sample_daily_at"`
}

type AlertsConfig struct {
	DedupWindow    Duration `yaml:"dedup_window"`
	RepeatInterval Duration `yaml:"repeat_interval"`
}

type RuleConfig struct {
	Name      string   `yaml:"name"`
	Type      string   `yaml:"type"`
	Mount     string   `yaml:"mount,omitempty"`
	Threshold float64  `yaml:"threshold"`
	For       Duration `yaml:"for"`
	Severity  string   `yaml:"severity"`
}

type NotifyConfig struct {
	Telegram TelegramConfig `yaml:"telegram"`
}

type HistoryConfig struct {
	DefaultDays int `yaml:"default_days"`
}

type IntegrationsConfig struct {
	VictoriaMetrics VictoriaMetricsConfig `yaml:"victoriametrics"`
	VMAgent         VMAgentConfig         `yaml:"vmagent"`
}

type VictoriaMetricsConfig struct {
	Enabled       bool   `yaml:"enabled"`
	Binary        string `yaml:"binary"`
	ListenAddr    string `yaml:"listen_addr"`
	DataPath      string `yaml:"data_path"`
	RetentionDays int    `yaml:"retention_days"`
}

type VMAgentConfig struct {
	Enabled        bool     `yaml:"enabled"`
	Binary         string   `yaml:"binary"`
	ListenAddr     string   `yaml:"listen_addr"`
	ScrapeInterval Duration `yaml:"scrape_interval"`
	RemoteWriteURL string   `yaml:"remote_write_url"`
}

type TelegramConfig struct {
	Enabled         bool     `yaml:"enabled"`
	BotToken        string   `yaml:"bot_token"`
	ChatID          string   `yaml:"chat_id"`
	CommandsEnabled bool     `yaml:"commands_enabled"`
	AllowedChatIDs  []string `yaml:"allowed_chat_ids"`
}

// Duration 支持 YAML 中 "60s", "5m" 等格式
type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var s string
	if err := value.Decode(&s); err != nil {
		return err
	}
	dur, err := time.ParseDuration(s)
	if err != nil {
		return fmt.Errorf("invalid duration %q: %w", s, err)
	}
	d.Duration = dur
	return nil
}

func (d Duration) MarshalYAML() (interface{}, error) {
	return d.Duration.String(), nil
}

// Load 从文件加载配置
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	setDefaults(cfg)
	return cfg, nil
}

func setDefaults(cfg *Config) {
	if cfg.Server.Listen == "" {
		cfg.Server.Listen = "127.0.0.1:18080"
	}
	if cfg.Collector.Interval.Duration == 0 {
		cfg.Collector.Interval.Duration = 60 * time.Second
	}
	if cfg.Collector.CPUSampleWindow.Duration == 0 {
		cfg.Collector.CPUSampleWindow.Duration = 2 * time.Second
	}
	if cfg.Storage.Dir == "" {
		cfg.Storage.Dir = "/var/lib/mini_monitor_server"
	}
	if cfg.Storage.KeepDaysLocal == 0 {
		if cfg.Storage.KeepDaysLegacy > 0 {
			cfg.Storage.KeepDaysLocal = cfg.Storage.KeepDaysLegacy
		} else {
			cfg.Storage.KeepDaysLocal = 90
		}
	}
	if cfg.Storage.DirSizeCheckInterval.Duration == 0 {
		cfg.Storage.DirSizeCheckInterval.Duration = 10 * time.Minute
	}
	if cfg.History.DefaultDays == 0 {
		cfg.History.DefaultDays = 7
	}
	if cfg.Alerts.DedupWindow.Duration == 0 {
		cfg.Alerts.DedupWindow.Duration = 30 * time.Minute
	}
	if cfg.Alerts.RepeatInterval.Duration == 0 {
		cfg.Alerts.RepeatInterval.Duration = 6 * time.Hour
	}
	if cfg.Integrations.VictoriaMetrics.Binary == "" {
		cfg.Integrations.VictoriaMetrics.Binary = "/usr/local/lib/mini_monitor_server/bin/victoria-metrics-prod"
	}
	if cfg.Integrations.VictoriaMetrics.ListenAddr == "" {
		cfg.Integrations.VictoriaMetrics.ListenAddr = "127.0.0.1:8428"
	}
	if cfg.Integrations.VictoriaMetrics.DataPath == "" {
		cfg.Integrations.VictoriaMetrics.DataPath = cfg.Storage.Dir + "/victoria-metrics"
	}
	if cfg.Integrations.VictoriaMetrics.RetentionDays == 0 {
		cfg.Integrations.VictoriaMetrics.RetentionDays = cfg.Storage.KeepDaysLocal
	}
	if cfg.Integrations.VMAgent.Binary == "" {
		cfg.Integrations.VMAgent.Binary = "/usr/local/lib/mini_monitor_server/bin/vmagent"
	}
	if cfg.Integrations.VMAgent.ListenAddr == "" {
		cfg.Integrations.VMAgent.ListenAddr = "127.0.0.1:8429"
	}
	if cfg.Integrations.VMAgent.ScrapeInterval.Duration == 0 {
		cfg.Integrations.VMAgent.ScrapeInterval.Duration = cfg.Collector.Interval.Duration
	}
	if cfg.Integrations.VMAgent.RemoteWriteURL == "" && cfg.Integrations.VictoriaMetrics.Enabled {
		cfg.Integrations.VMAgent.RemoteWriteURL = "http://" + normalizeListenAddr(cfg.Integrations.VictoriaMetrics.ListenAddr) + "/api/v1/write"
	}
}

// Validate 校验配置合法性
func Validate(cfg *Config) error {
	if cfg.Collector.Interval.Duration < time.Second {
		return fmt.Errorf("collector.interval must be >= 1s")
	}
	if cfg.Collector.CPUSampleWindow.Duration < 100*time.Millisecond {
		return fmt.Errorf("collector.cpu_sample_window must be >= 100ms")
	}
	if cfg.Integrations.VMAgent.ScrapeInterval.Duration > 0 && cfg.Integrations.VMAgent.ScrapeInterval.Duration < time.Second {
		return fmt.Errorf("integrations.vmagent.scrape_interval must be >= 1s")
	}
	if cfg.Storage.DirSizeCheckInterval.Duration > 0 && cfg.Storage.DirSizeCheckInterval.Duration < time.Second {
		return fmt.Errorf("storage.dir_size_check_interval must be >= 1s")
	}
	if cfg.Disk.SampleDailyAt != "" {
		if _, err := time.Parse("15:04", cfg.Disk.SampleDailyAt); err != nil {
			return fmt.Errorf("disk.sample_daily_at must be HH:MM: %w", err)
		}
	}
	knownMounts := make(map[string]struct{}, len(cfg.Disk.Mounts))
	for _, mount := range cfg.Disk.Mounts {
		knownMounts[mount] = struct{}{}
	}
	ruleNames := make(map[string]struct{}, len(cfg.Rules))
	for i, r := range cfg.Rules {
		if r.Name == "" {
			return fmt.Errorf("rules[%d].name is required", i)
		}
		if _, exists := ruleNames[r.Name]; exists {
			return fmt.Errorf("rules[%d].name %q is duplicated", i, r.Name)
		}
		ruleNames[r.Name] = struct{}{}
		switch r.Type {
		case "cpu_used_percent", "memory_used_percent", "disk_used_percent", "storage_dir_size_mb":
		default:
			return fmt.Errorf("rules[%d].type %q is unsupported", i, r.Type)
		}
		if r.Type == "disk_used_percent" && r.Mount == "" {
			return fmt.Errorf("rules[%d] disk rule requires mount", i)
		}
		if r.Type == "disk_used_percent" {
			if _, ok := knownMounts[r.Mount]; !ok {
				return fmt.Errorf("rules[%d].mount %q is not configured in disk.mounts", i, r.Mount)
			}
		}
		if r.Threshold <= 0 {
			return fmt.Errorf("rules[%d].threshold must be > 0", i)
		}
		if r.Type != "storage_dir_size_mb" && r.Threshold > 100 {
			return fmt.Errorf("rules[%d].threshold must be in (0, 100]", i)
		}
		if r.For.Duration <= 0 {
			return fmt.Errorf("rules[%d].for must be > 0", i)
		}
		if r.Severity == "" {
			return fmt.Errorf("rules[%d].severity is required", i)
		}
	}
	if cfg.Storage.DirSizeAlertMB > 0 {
		if _, exists := ruleNames[StorageDirAlertRuleName]; exists {
			return fmt.Errorf("rules.%s is reserved when storage.dir_size_alert_mb is enabled", StorageDirAlertRuleName)
		}
	}
	if cfg.Notify.Telegram.Enabled {
		if cfg.Notify.Telegram.BotToken == "" {
			return fmt.Errorf("notify.telegram.bot_token is required when enabled")
		}
		if cfg.Notify.Telegram.ChatID == "" {
			return fmt.Errorf("notify.telegram.chat_id is required when enabled")
		}
		if cfg.Notify.Telegram.CommandsEnabled && len(cfg.Notify.Telegram.AllowedChatIDs) == 0 {
			return fmt.Errorf("notify.telegram.allowed_chat_ids is required when commands are enabled")
		}
	}
	if cfg.Integrations.VictoriaMetrics.Enabled {
		if cfg.Integrations.VictoriaMetrics.ListenAddr == "" {
			return fmt.Errorf("integrations.victoriametrics.listen_addr is required when enabled")
		}
		if cfg.Integrations.VictoriaMetrics.DataPath == "" {
			return fmt.Errorf("integrations.victoriametrics.data_path is required when enabled")
		}
		if cfg.Integrations.VictoriaMetrics.RetentionDays <= 0 {
			return fmt.Errorf("integrations.victoriametrics.retention_days must be > 0 when enabled")
		}
	}
	if cfg.Integrations.VMAgent.Enabled {
		if cfg.Integrations.VMAgent.ListenAddr == "" {
			return fmt.Errorf("integrations.vmagent.listen_addr is required when enabled")
		}
		if cfg.Integrations.VMAgent.RemoteWriteURL == "" {
			return fmt.Errorf("integrations.vmagent.remote_write_url is required when enabled")
		}
	}
	return nil
}

func normalizeListenAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "" || host == "::" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}
