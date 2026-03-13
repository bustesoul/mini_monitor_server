package integration

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"mini_monitor_server/internal/config"
)

func TestWriteVMAgentConfig(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Server: config.ServerConfig{
			Listen: "127.0.0.1:18080",
		},
		Collector: config.CollectorConfig{
			Interval: duration(60 * time.Second),
		},
		Storage: config.StorageConfig{
			Dir: dir,
		},
		Integrations: config.IntegrationsConfig{
			VMAgent: config.VMAgentConfig{
				Enabled:        true,
				ListenAddr:     "127.0.0.1:8429",
				ScrapeInterval: duration(30 * time.Second),
				RemoteWriteURL: "http://127.0.0.1:8428/api/v1/write",
			},
		},
	}
	path, err := writeVMAgentConfig(cfg, filepath.Join(dir, "integrations"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	for _, want := range []string{
		"scrape_interval: 30s",
		"job_name: mini_monitor_server",
		"- 127.0.0.1:18080",
		"metrics_path: /metrics",
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("vmagent config missing %q\n%s", want, text)
		}
	}
}

func TestVictoriaMetricsSpec(t *testing.T) {
	cfg := &config.Config{
		Storage: config.StorageConfig{
			KeepDaysLocal: 90,
		},
		Integrations: config.IntegrationsConfig{
			VictoriaMetrics: config.VictoriaMetricsConfig{
				Enabled:       true,
				Binary:        "victoria-metrics-prod",
				ListenAddr:    "127.0.0.1:8428",
				DataPath:      "/tmp/victoria",
				RetentionDays: 30,
			},
		},
	}
	spec := victoriaMetricsSpec(cfg)
	if spec.binary != "victoria-metrics-prod" {
		t.Fatalf("binary = %q", spec.binary)
	}
	joined := strings.Join(spec.args, " ")
	for _, want := range []string{
		"-storageDataPath=/tmp/victoria",
		"-httpListenAddr=127.0.0.1:8428",
		"-retentionPeriod=30d",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("args missing %q: %s", want, joined)
		}
	}
}

func duration(d time.Duration) config.Duration {
	return config.Duration{Duration: d}
}
