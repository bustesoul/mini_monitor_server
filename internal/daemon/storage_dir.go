package daemon

import (
	"log/slog"
	"sync"
	"time"

	"mini_monitor_server/internal/storage"
)

type storageDirSizer struct {
	path     string
	interval time.Duration

	mu       sync.Mutex
	lastAt   time.Time
	lastSize uint64
}

func newStorageDirSizer(path string, interval time.Duration) *storageDirSizer {
	return &storageDirSizer{
		path:     path,
		interval: interval,
	}
}

func (s *storageDirSizer) Size(now time.Time) uint64 {
	if s == nil {
		return 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.lastAt.IsZero() && s.interval > 0 && now.Sub(s.lastAt) < s.interval {
		return s.lastSize
	}

	size, err := storage.DirSizeBytes(s.path)
	if err != nil {
		slog.Warn("measure storage dir size failed", "path", s.path, "error", err)
		return s.lastSize
	}

	s.lastAt = now
	s.lastSize = size
	return size
}
