package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server    ServerConfig    `yaml:"server"`
	Collector CollectorConfig `yaml:"collector"`
	Storage   StorageConfig   `yaml:"storage"`
	Network   NetworkConfig   `yaml:"network"`
	Disk      DiskConfig      `yaml:"disk"`
	Alerts    AlertsConfig    `yaml:"alerts"`
	Rules     []RuleConfig    `yaml:"rules"`
	Notify    NotifyConfig    `yaml:"notify"`
	Report    ReportConfig    `yaml:"report"`
}

type ServerConfig struct {
	Listen string `yaml:"listen"`
}

type CollectorConfig struct {
	Interval        Duration `yaml:"interval"`
	CPUSampleWindow Duration `yaml:"cpu_sample_window"`
}

type StorageConfig struct {
	Dir      string `yaml:"dir"`
	KeepDays int    `yaml:"keep_days"`
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

type TelegramConfig struct {
	Enabled         bool     `yaml:"enabled"`
	BotToken        string   `yaml:"bot_token"`
	ChatID          string   `yaml:"chat_id"`
	CommandsEnabled bool     `yaml:"commands_enabled"`
	AllowedChatIDs  []string `yaml:"allowed_chat_ids"`
}

type ReportConfig struct {
	IncludeHistoryDays int `yaml:"include_history_days"`
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
	if cfg.Storage.KeepDays == 0 {
		cfg.Storage.KeepDays = 90
	}
	if cfg.Alerts.DedupWindow.Duration == 0 {
		cfg.Alerts.DedupWindow.Duration = 30 * time.Minute
	}
	if cfg.Alerts.RepeatInterval.Duration == 0 {
		cfg.Alerts.RepeatInterval.Duration = 6 * time.Hour
	}
	if cfg.Report.IncludeHistoryDays == 0 {
		cfg.Report.IncludeHistoryDays = 7
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
		case "cpu_used_percent", "memory_used_percent", "disk_used_percent":
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
		if r.Threshold <= 0 || r.Threshold > 100 {
			return fmt.Errorf("rules[%d].threshold must be in (0, 100]", i)
		}
		if r.For.Duration <= 0 {
			return fmt.Errorf("rules[%d].for must be > 0", i)
		}
		if r.Severity == "" {
			return fmt.Errorf("rules[%d].severity is required", i)
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
	return nil
}
