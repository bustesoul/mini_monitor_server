package rule

import (
	"log/slog"
	"sync"
	"time"

	"mini_monitor_server/internal/config"
	"mini_monitor_server/internal/model"
)

// Engine 规则引擎，评估采集结果并管理告警状态机
type Engine struct {
	rules  []config.RuleConfig
	states map[string]*model.RuleRuntimeState
	mu     sync.RWMutex
}

func NewEngine(rules []config.RuleConfig) *Engine {
	states := make(map[string]*model.RuleRuntimeState, len(rules))
	for _, r := range rules {
		states[r.Name] = &model.RuleRuntimeState{
			Name:   r.Name,
			Status: "normal",
		}
	}
	return &Engine{rules: rules, states: states}
}

// RestoreStates 从持久化状态恢复
func (e *Engine) RestoreStates(saved map[string]model.RuleRuntimeState) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for name, s := range saved {
		if _, ok := e.states[name]; ok {
			copy := s
			e.states[name] = &copy
		}
	}
}

// States 返回当前所有规则状态
func (e *Engine) States() map[string]model.RuleRuntimeState {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make(map[string]model.RuleRuntimeState, len(e.states))
	for k, v := range e.states {
		result[k] = *v
	}
	return result
}

// Evaluate 评估快照，返回需要发送的告警事件
func (e *Engine) Evaluate(snap *model.Snapshot, now time.Time) []*model.AlertEvent {
	e.mu.Lock()
	defer e.mu.Unlock()

	var events []*model.AlertEvent
	for _, rule := range e.rules {
		value := extractValue(snap, rule)
		exceeded := value > rule.Threshold
		state := e.states[rule.Name]

		switch state.Status {
		case "normal":
			if exceeded {
				state.Status = "pending"
				state.PendingSince = &now
				slog.Debug("rule pending", "rule", rule.Name, "value", value)
			}

		case "pending":
			if !exceeded {
				state.Status = "normal"
				state.PendingSince = nil
				slog.Debug("rule back to normal", "rule", rule.Name, "value", value)
			} else if state.PendingSince != nil && now.Sub(*state.PendingSince) >= rule.For.Duration {
				state.Status = "firing"
				state.FiringSince = &now
				events = append(events, &model.AlertEvent{
					Timestamp: now,
					Rule:      rule.Name,
					Status:    "firing",
					Value:     value,
					Severity:  rule.Severity,
				})
				slog.Info("rule firing", "rule", rule.Name, "value", value)
			}

		case "firing":
			if !exceeded {
				state.Status = "normal"
				state.PendingSince = nil
				state.FiringSince = nil
				events = append(events, &model.AlertEvent{
					Timestamp: now,
					Rule:      rule.Name,
					Status:    "recovered",
					Value:     value,
					Severity:  rule.Severity,
				})
				slog.Info("rule recovered", "rule", rule.Name, "value", value)
			}
		}
	}
	return events
}

// FiringRules 返回当前处于 firing 状态的规则名列表
func (e *Engine) FiringRules() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var names []string
	for _, s := range e.states {
		if s.Status == "firing" {
			names = append(names, s.Name)
		}
	}
	return names
}

func extractValue(snap *model.Snapshot, rule config.RuleConfig) float64 {
	switch rule.Type {
	case "cpu_used_percent":
		return snap.CPU.UsagePercent
	case "memory_used_percent":
		return snap.Memory.UsedPercent
	case "disk_used_percent":
		for _, d := range snap.Disks {
			if d.Mount == rule.Mount {
				return d.UsedPercent
			}
		}
	case "storage_dir_size_mb":
		return float64(snap.StorageDirSizeBytes) / (1024 * 1024)
	}
	return 0
}
