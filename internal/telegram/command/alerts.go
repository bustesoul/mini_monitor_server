package command

import (
	"context"
	"fmt"

	"mini_monitor_server/internal/model"
	"mini_monitor_server/internal/storage"
)

type AlertsCmd struct {
	store *storage.Storage
}

func (c *AlertsCmd) Name() string        { return "alerts" }
func (c *AlertsCmd) Description() string  { return "Show recent alerts" }

func (c *AlertsCmd) Execute(_ context.Context, _ string) (string, error) {
	alerts, err := c.store.ReadAlerts(10)
	if err != nil {
		return "", err
	}
	if len(alerts) == 0 {
		return "No alerts.", nil
	}
	result := "Recent alerts:\n"
	for _, a := range alerts {
		result += formatAlertLine(&a)
	}
	return result, nil
}

func formatAlertLine(a *model.AlertEvent) string {
	return fmt.Sprintf("  [%s] %s %s value=%.1f%% at %s\n",
		a.Status, a.Severity, a.Rule, a.Value,
		a.Timestamp.Format("01-02 15:04"))
}
