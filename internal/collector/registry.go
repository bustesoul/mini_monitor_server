package collector

import (
	"context"
	"fmt"
	"sync"

	"mini_monitor_server/internal/model"
)

// Collector 采集器插件接口
type Collector interface {
	Name() string
	Collect(ctx context.Context, snap *model.Snapshot) error
}

// Registry 采集器注册表
type Registry struct {
	mu         sync.RWMutex
	collectors map[string]Collector
}

func NewRegistry() *Registry {
	return &Registry{collectors: make(map[string]Collector)}
}

func (r *Registry) Register(c Collector) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.collectors[c.Name()] = c
}

func (r *Registry) Get(name string) (Collector, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.collectors[name]
	return c, ok
}

func (r *Registry) CollectAll(ctx context.Context, snap *model.Snapshot) []error {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var errs []error
	for _, c := range r.collectors {
		if err := c.Collect(ctx, snap); err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", c.Name(), err))
		}
	}
	return errs
}

func (r *Registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.collectors))
	for name := range r.collectors {
		names = append(names, name)
	}
	return names
}
