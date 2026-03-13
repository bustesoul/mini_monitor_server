package integration

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"mini_monitor_server/internal/config"
)

type Manager struct {
	cfg       *config.Config
	once      sync.Once
	cancel    context.CancelFunc
	processes []*supervisedProcess
}

func NewManager(cfg *config.Config) *Manager {
	return &Manager{cfg: cfg}
}

func (m *Manager) Enabled() bool {
	if m == nil || m.cfg == nil {
		return false
	}
	return m.cfg.Integrations.VictoriaMetrics.Enabled || m.cfg.Integrations.VMAgent.Enabled
}

func (m *Manager) Start(ctx context.Context) {
	if !m.Enabled() {
		return
	}

	m.once.Do(func() {
		runCtx, cancel := context.WithCancel(ctx)
		m.cancel = cancel

		if err := os.MkdirAll(m.integrationDir(), 0755); err != nil {
			slog.Error("create integration dir failed", "error", err)
			return
		}

		if m.cfg.Integrations.VictoriaMetrics.Enabled {
			spec := victoriaMetricsSpec(m.cfg)
			proc := newSupervisedProcess("victoriametrics", spec)
			m.processes = append(m.processes, proc)
			proc.Start(runCtx)
		}

		if m.cfg.Integrations.VMAgent.Enabled {
			cfgPath, err := writeVMAgentConfig(m.cfg, m.integrationDir())
			if err != nil {
				slog.Error("write vmagent config failed", "error", err)
				return
			}
			spec := vmagentSpec(m.cfg, cfgPath)
			proc := newSupervisedProcess("vmagent", spec)
			m.processes = append(m.processes, proc)
			proc.Start(runCtx)
		}
	})
}

func (m *Manager) Stop() {
	if m == nil {
		return
	}
	if m.cancel != nil {
		m.cancel()
	}
	for _, proc := range m.processes {
		proc.Stop(5 * time.Second)
	}
}

func (m *Manager) integrationDir() string {
	return filepath.Join(m.cfg.Storage.Dir, "integrations")
}
