package command

import (
	"mini_monitor_server/internal/config"
	"mini_monitor_server/internal/model"
	"mini_monitor_server/internal/rule"
	"mini_monitor_server/internal/storage"
)

// RegisterAll 注册所有内置命令
func RegisterAll(reg *Registry, getSnapshot func() *model.Snapshot, engine *rule.Engine, store *storage.Storage, cfg *config.Config) {
	reg.Register(&ReportCmd{getSnapshot: getSnapshot, engine: engine})
	reg.Register(&CPUCmd{getSnapshot: getSnapshot})
	reg.Register(&MemCmd{getSnapshot: getSnapshot})
	reg.Register(&DiskCmd{getSnapshot: getSnapshot})
	reg.Register(&NetCmd{getSnapshot: getSnapshot})
	reg.Register(&AlertsCmd{store: store})

	helpCmd := &HelpCmd{registry: reg}
	reg.Register(helpCmd)
}
