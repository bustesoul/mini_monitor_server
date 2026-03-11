package notifier

import (
	"context"
	"fmt"
	"log/slog"

	"mini_monitor_server/internal/model"
)

// LogNotifier 日志告警通知
type LogNotifier struct{}

func NewLogNotifier() *LogNotifier { return &LogNotifier{} }

func (l *LogNotifier) Name() string { return "log" }

func (l *LogNotifier) SendAlert(_ context.Context, evt *model.AlertEvent) error {
	slog.Warn(fmt.Sprintf("[ALERT] [%s] %s value=%.1f%%", evt.Severity, evt.Rule, evt.Value))
	return nil
}

func (l *LogNotifier) SendRecovery(_ context.Context, evt *model.AlertEvent) error {
	slog.Info(fmt.Sprintf("[RECOVERED] %s value=%.1f%%", evt.Rule, evt.Value))
	return nil
}

func (l *LogNotifier) Close() error { return nil }
