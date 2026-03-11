package command

import (
	"context"
	"sync"
)

// BotCommand Bot 命令插件接口
type BotCommand interface {
	Name() string
	Description() string
	Execute(ctx context.Context, args string) (string, error)
}

// Registry 命令注册表
type Registry struct {
	mu       sync.RWMutex
	commands map[string]BotCommand
}

func NewRegistry() *Registry {
	return &Registry{commands: make(map[string]BotCommand)}
}

func (r *Registry) Register(cmd BotCommand) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.commands[cmd.Name()] = cmd
}

func (r *Registry) Get(name string) (BotCommand, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.commands[name]
	return c, ok
}

func (r *Registry) All() []BotCommand {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]BotCommand, 0, len(r.commands))
	for _, c := range r.commands {
		list = append(list, c)
	}
	return list
}
