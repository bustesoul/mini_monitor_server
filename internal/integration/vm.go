package integration

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"mini_monitor_server/internal/config"
)

func victoriaMetricsSpec(cfg *config.Config) processSpec {
	retentionDays := cfg.Integrations.VictoriaMetrics.RetentionDays
	if retentionDays < 1 {
		retentionDays = 1
	}

	return processSpec{
		binary: cfg.Integrations.VictoriaMetrics.Binary,
		args: []string{
			"-storageDataPath=" + cfg.Integrations.VictoriaMetrics.DataPath,
			"-httpListenAddr=" + cfg.Integrations.VictoriaMetrics.ListenAddr,
			fmt.Sprintf("-retentionPeriod=%dd", retentionDays),
		},
	}
}

func vmagentSpec(cfg *config.Config, scrapeConfigPath string) processSpec {
	return processSpec{
		binary: cfg.Integrations.VMAgent.Binary,
		args: []string{
			"-httpListenAddr=" + cfg.Integrations.VMAgent.ListenAddr,
			"-promscrape.config=" + scrapeConfigPath,
			"-remoteWrite.url=" + normalizeRemoteWriteURL(cfg.Integrations.VMAgent.RemoteWriteURL),
		},
	}
}

type vmagentConfigFile struct {
	Global        vmagentGlobalConfig   `yaml:"global"`
	ScrapeConfigs []vmagentScrapeConfig `yaml:"scrape_configs"`
}

type vmagentGlobalConfig struct {
	ScrapeInterval string `yaml:"scrape_interval"`
}

type vmagentScrapeConfig struct {
	JobName       string                `yaml:"job_name"`
	MetricsPath   string                `yaml:"metrics_path,omitempty"`
	StaticConfigs []vmagentStaticConfig `yaml:"static_configs"`
}

type vmagentStaticConfig struct {
	Targets []string `yaml:"targets"`
}

func writeVMAgentConfig(cfg *config.Config, dir string) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	file := vmagentConfigFile{
		Global: vmagentGlobalConfig{
			ScrapeInterval: cfg.Integrations.VMAgent.ScrapeInterval.String(),
		},
		ScrapeConfigs: []vmagentScrapeConfig{
			{
				JobName:     "mini_monitor_server",
				MetricsPath: "/metrics",
				StaticConfigs: []vmagentStaticConfig{
					{Targets: []string{normalizeTarget(cfg.Server.Listen)}},
				},
			},
		},
	}

	data, err := yaml.Marshal(&file)
	if err != nil {
		return "", err
	}

	path := filepath.Join(dir, "vmagent-promscrape.yml")
	if err := os.WriteFile(path, data, 0644); err != nil {
		return "", err
	}
	return path, nil
}

func normalizeTarget(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "" || host == "::" || host == "0.0.0.0" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

func normalizeRemoteWriteURL(raw string) string {
	if raw == "" {
		return ""
	}
	if parsed, err := url.Parse(raw); err == nil && parsed.Scheme != "" {
		return raw
	}
	return "http://" + normalizeTarget(raw) + "/api/v1/write"
}
