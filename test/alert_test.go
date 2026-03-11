package test

import (
	"context"
	"sync"
	"testing"
	"time"

	"mini_monitor_server/internal/alert"
	"mini_monitor_server/internal/model"
	"mini_monitor_server/internal/notifier"
	"mini_monitor_server/internal/storage"
)

func TestAlertManagerDedup(t *testing.T) {
	dir := t.TempDir()
	store, _ := storage.New(dir)
	reg := notifier.NewRegistry()
	mock := &mockNotifier{}
	reg.Register(mock)

	mgr := alert.NewManager(30*time.Minute, 6*time.Hour, reg, store)

	evt := &model.AlertEvent{
		Timestamp: time.Now(),
		Rule:      "cpu_high",
		Status:    "firing",
		Value:     95,
		Severity:  "warning",
	}

	// 第一次发送
	mgr.Process(context.Background(), []*model.AlertEvent{evt})
	if mock.alertCount() != 1 {
		t.Fatalf("first send: alertCount = %d, want 1", mock.alertCount())
	}

	// 立即再发，应被去重
	mgr.Process(context.Background(), []*model.AlertEvent{evt})
	if mock.alertCount() != 1 {
		t.Fatalf("deduped send: alertCount = %d, want 1", mock.alertCount())
	}
}

func TestAlertManagerRecovery(t *testing.T) {
	dir := t.TempDir()
	store, _ := storage.New(dir)
	reg := notifier.NewRegistry()
	mock := &mockNotifier{}
	reg.Register(mock)

	mgr := alert.NewManager(30*time.Minute, 6*time.Hour, reg, store)

	// firing
	mgr.Process(context.Background(), []*model.AlertEvent{
		{Rule: "r1", Status: "firing", Value: 90, Severity: "warning", Timestamp: time.Now()},
	})
	// recovery
	mgr.Process(context.Background(), []*model.AlertEvent{
		{Rule: "r1", Status: "recovered", Value: 70, Severity: "warning", Timestamp: time.Now()},
	})

	if mock.alertCount() != 1 {
		t.Errorf("alertCount = %d, want 1", mock.alertCount())
	}
	if mock.recoveryCount() != 1 {
		t.Errorf("recoveryCount = %d, want 1", mock.recoveryCount())
	}
}

func TestAlertManagerPersistsEvents(t *testing.T) {
	dir := t.TempDir()
	store, _ := storage.New(dir)
	reg := notifier.NewRegistry()
	reg.Register(&mockNotifier{})

	mgr := alert.NewManager(0, 6*time.Hour, reg, store)
	mgr.Process(context.Background(), []*model.AlertEvent{
		{Rule: "r1", Status: "firing", Value: 91, Severity: "critical", Timestamp: time.Now()},
	})

	alerts, _ := store.ReadAlerts(10)
	if len(alerts) != 1 {
		t.Fatalf("persisted alerts = %d, want 1", len(alerts))
	}
	if alerts[0].Rule != "r1" {
		t.Errorf("persisted rule = %q, want %q", alerts[0].Rule, "r1")
	}
}

// --- mock notifier ---

type mockNotifier struct {
	mu        sync.Mutex
	alerts    int
	recoveries int
}

func (m *mockNotifier) Name() string { return "mock" }

func (m *mockNotifier) SendAlert(_ context.Context, _ *model.AlertEvent) error {
	m.mu.Lock()
	m.alerts++
	m.mu.Unlock()
	return nil
}

func (m *mockNotifier) SendRecovery(_ context.Context, _ *model.AlertEvent) error {
	m.mu.Lock()
	m.recoveries++
	m.mu.Unlock()
	return nil
}

func (m *mockNotifier) Close() error { return nil }

func (m *mockNotifier) alertCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.alerts
}

func (m *mockNotifier) recoveryCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.recoveries
}
