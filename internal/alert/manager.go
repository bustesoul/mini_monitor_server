package alert

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"mini_monitor_server/internal/model"
	"mini_monitor_server/internal/notifier"
	"mini_monitor_server/internal/storage"
)

// Manager 告警管理器，负责去重、重复提醒、分发到 Notifier
type Manager struct {
	dedupWindow    time.Duration
	repeatInterval time.Duration
	notifiers      *notifier.Registry
	store          *storage.Storage
	mu             sync.Mutex
	lastSent       map[string]time.Time // rule -> last send time
}

func NewManager(dedupWindow, repeatInterval time.Duration, notifiers *notifier.Registry, store *storage.Storage) *Manager {
	return &Manager{
		dedupWindow:    dedupWindow,
		repeatInterval: repeatInterval,
		notifiers:      notifiers,
		store:          store,
		lastSent:       make(map[string]time.Time),
	}
}

// RestoreLastSent 从规则状态恢复 lastSent 时间
func (m *Manager) RestoreLastSent(rules map[string]model.RuleRuntimeState) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for name, state := range rules {
		if state.LastSentAt != nil {
			m.lastSent[name] = *state.LastSentAt
		}
	}
}

// Process 处理规则引擎产生的告警事件
func (m *Manager) Process(ctx context.Context, events []*model.AlertEvent) {
	for _, evt := range events {
		if evt.Status == "recovered" {
			m.sendRecovery(ctx, evt)
			continue
		}
		m.sendAlert(ctx, evt)
	}
}

// CheckRepeat 检查 firing 状态的规则是否需要重复提醒
func (m *Manager) CheckRepeat(ctx context.Context, firingRules []string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for _, name := range firingRules {
		last, ok := m.lastSent[name]
		if !ok {
			continue
		}
		if now.Sub(last) >= m.repeatInterval {
			evt := &model.AlertEvent{
				Timestamp: now,
				Rule:      name,
				Status:    "firing",
				Severity:  "reminder",
				Message:   "still firing",
			}
			m.dispatch(ctx, evt, false)
			m.lastSent[name] = now
		}
	}
}

func (m *Manager) LastSentSnapshot() map[string]time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make(map[string]time.Time, len(m.lastSent))
	for name, ts := range m.lastSent {
		result[name] = ts
	}
	return result
}

func (m *Manager) sendAlert(ctx context.Context, evt *model.AlertEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	if last, ok := m.lastSent[evt.Rule]; ok {
		if now.Sub(last) < m.dedupWindow {
			slog.Debug("alert deduped", "rule", evt.Rule)
			return
		}
	}

	m.dispatch(ctx, evt, true)
	m.lastSent[evt.Rule] = now
}

func (m *Manager) sendRecovery(ctx context.Context, evt *model.AlertEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.dispatch(ctx, evt, true)
	delete(m.lastSent, evt.Rule)
}

func (m *Manager) dispatch(ctx context.Context, evt *model.AlertEvent, persist bool) {
	if persist {
		if err := m.store.AppendAlert(evt); err != nil {
			slog.Error("persist alert failed", "error", err)
		}
	}

	for _, n := range m.notifiers.All() {
		var err error
		if evt.Status == "recovered" {
			err = n.SendRecovery(ctx, evt)
		} else {
			err = n.SendAlert(ctx, evt)
		}
		if err != nil {
			slog.Error("notify failed", "notifier", n.Name(), "error", err)
		}
	}
}
