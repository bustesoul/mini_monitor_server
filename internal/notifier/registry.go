package notifier

import (
	"context"
	"sync"

	"mini_monitor_server/internal/model"
)

// Notifier 通知器插件接口
type Notifier interface {
	Name() string
	SendAlert(ctx context.Context, evt *model.AlertEvent) error
	SendRecovery(ctx context.Context, evt *model.AlertEvent) error
	Close() error
}

// Registry 通知器注册表
type Registry struct {
	mu        sync.RWMutex
	notifiers map[string]Notifier
}

func NewRegistry() *Registry {
	return &Registry{notifiers: make(map[string]Notifier)}
}

func (r *Registry) Register(n Notifier) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notifiers[n.Name()] = n
}

func (r *Registry) Get(name string) (Notifier, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n, ok := r.notifiers[name]
	return n, ok
}

func (r *Registry) All() []Notifier {
	r.mu.RLock()
	defer r.mu.RUnlock()
	list := make([]Notifier, 0, len(r.notifiers))
	for _, n := range r.notifiers {
		list = append(list, n)
	}
	return list
}

func (r *Registry) CloseAll() {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, n := range r.notifiers {
		n.Close()
	}
}
