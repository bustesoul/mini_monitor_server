package test

import (
	"testing"
	"time"

	"mini_monitor_server/internal/config"
	"mini_monitor_server/internal/model"
	"mini_monitor_server/internal/rule"
)

func TestRuleEngineNormalToFiring(t *testing.T) {
	rules := []config.RuleConfig{
		{Name: "cpu_high", Type: "cpu_used_percent", Threshold: 80, Severity: "warning",
			For: config.Duration{Duration: 2 * time.Minute}},
	}
	engine := rule.NewEngine(rules)

	// 第一次超阈值：normal → pending
	snap := makeSnap(85, 0, nil)
	t0 := time.Now()
	events := engine.Evaluate(snap, t0)
	if len(events) != 0 {
		t.Fatalf("first eval: got %d events, want 0 (should be pending)", len(events))
	}
	assertState(t, engine, "cpu_high", "pending")

	// 1 分钟后仍超阈值，但不到 for 时间：仍 pending
	events = engine.Evaluate(snap, t0.Add(1*time.Minute))
	if len(events) != 0 {
		t.Fatalf("second eval: got %d events, want 0", len(events))
	}
	assertState(t, engine, "cpu_high", "pending")

	// 3 分钟后仍超阈值，超过 for：pending → firing
	events = engine.Evaluate(snap, t0.Add(3*time.Minute))
	if len(events) != 1 {
		t.Fatalf("third eval: got %d events, want 1", len(events))
	}
	if events[0].Status != "firing" {
		t.Errorf("event status = %q, want %q", events[0].Status, "firing")
	}
	if events[0].Rule != "cpu_high" {
		t.Errorf("event rule = %q, want %q", events[0].Rule, "cpu_high")
	}
	assertState(t, engine, "cpu_high", "firing")
}

func TestRuleEngineRecovery(t *testing.T) {
	rules := []config.RuleConfig{
		{Name: "mem_high", Type: "memory_used_percent", Threshold: 80, Severity: "critical",
			For: config.Duration{Duration: 0}}, // for=0 表示立即 firing
	}
	engine := rule.NewEngine(rules)
	now := time.Now()

	// 超阈值 → pending
	engine.Evaluate(makeSnap(0, 90, nil), now)
	// for=0 所以同一秒再评估应该 firing
	events := engine.Evaluate(makeSnap(0, 90, nil), now)
	if len(events) != 1 || events[0].Status != "firing" {
		t.Fatalf("expected firing, got %v", events)
	}

	// 恢复
	events = engine.Evaluate(makeSnap(0, 70, nil), now.Add(time.Minute))
	if len(events) != 1 || events[0].Status != "recovered" {
		t.Fatalf("expected recovered, got %v", events)
	}
	assertState(t, engine, "mem_high", "normal")
}

func TestRuleEnginePendingBackToNormal(t *testing.T) {
	rules := []config.RuleConfig{
		{Name: "cpu_high", Type: "cpu_used_percent", Threshold: 80, Severity: "warning",
			For: config.Duration{Duration: 5 * time.Minute}},
	}
	engine := rule.NewEngine(rules)
	now := time.Now()

	// 超阈值 → pending
	engine.Evaluate(makeSnap(85, 0, nil), now)
	assertState(t, engine, "cpu_high", "pending")

	// 恢复到阈值以下 → normal（不产生 recovered 事件，因为从未 firing）
	events := engine.Evaluate(makeSnap(70, 0, nil), now.Add(time.Minute))
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
	assertState(t, engine, "cpu_high", "normal")
}

func TestRuleEngineDiskRule(t *testing.T) {
	rules := []config.RuleConfig{
		{Name: "disk_root", Type: "disk_used_percent", Mount: "/", Threshold: 90, Severity: "critical",
			For: config.Duration{Duration: 0}},
	}
	engine := rule.NewEngine(rules)
	now := time.Now()

	disks := []model.DiskStat{{Mount: "/", UsedPercent: 95}}
	engine.Evaluate(makeSnap(0, 0, disks), now)
	events := engine.Evaluate(makeSnap(0, 0, disks), now)
	if len(events) != 1 || events[0].Rule != "disk_root" {
		t.Fatalf("expected disk_root firing, got %v", events)
	}
}

func TestRuleEngineFiringRules(t *testing.T) {
	rules := []config.RuleConfig{
		{Name: "r1", Type: "cpu_used_percent", Threshold: 50, Severity: "warning",
			For: config.Duration{Duration: 0}},
		{Name: "r2", Type: "memory_used_percent", Threshold: 50, Severity: "warning",
			For: config.Duration{Duration: 0}},
	}
	engine := rule.NewEngine(rules)
	now := time.Now()

	// 仅 CPU 超阈值
	snap := makeSnap(80, 30, nil)
	engine.Evaluate(snap, now)
	engine.Evaluate(snap, now)

	firing := engine.FiringRules()
	if len(firing) != 1 || firing[0] != "r1" {
		t.Errorf("FiringRules() = %v, want [r1]", firing)
	}
}

func TestRuleEngineRestoreStates(t *testing.T) {
	rules := []config.RuleConfig{
		{Name: "cpu_high", Type: "cpu_used_percent", Threshold: 80, Severity: "warning",
			For: config.Duration{Duration: 0}},
	}
	engine := rule.NewEngine(rules)

	firing := time.Now().Add(-10 * time.Minute)
	engine.RestoreStates(map[string]model.RuleRuntimeState{
		"cpu_high": {Name: "cpu_high", Status: "firing", FiringSince: &firing},
	})

	assertState(t, engine, "cpu_high", "firing")

	// 恢复事件
	events := engine.Evaluate(makeSnap(50, 0, nil), time.Now())
	if len(events) != 1 || events[0].Status != "recovered" {
		t.Fatalf("expected recovered after restore, got %v", events)
	}
}

func assertState(t *testing.T, engine *rule.Engine, name, want string) {
	t.Helper()
	states := engine.States()
	s, ok := states[name]
	if !ok {
		t.Fatalf("state %q not found", name)
	}
	if s.Status != want {
		t.Errorf("state %q = %q, want %q", name, s.Status, want)
	}
}

func makeSnap(cpu, mem float64, disks []model.DiskStat) *model.Snapshot {
	return &model.Snapshot{
		CPU:    model.CPUStat{UsagePercent: cpu},
		Memory: model.MemoryStat{UsedPercent: mem},
		Disks:  disks,
	}
}
