package test

import (
	"context"
	"testing"

	"mini_monitor_server/internal/collector"
	"mini_monitor_server/internal/model"
)

func TestCollectorRegistry(t *testing.T) {
	reg := collector.NewRegistry()

	if len(reg.List()) != 0 {
		t.Fatalf("empty registry List() = %d, want 0", len(reg.List()))
	}

	reg.Register(&fakeCollector{name: "a"})
	reg.Register(&fakeCollector{name: "b"})

	if len(reg.List()) != 2 {
		t.Fatalf("List() = %d, want 2", len(reg.List()))
	}

	c, ok := reg.Get("a")
	if !ok || c.Name() != "a" {
		t.Errorf("Get(a) = %v, %v", c, ok)
	}

	_, ok = reg.Get("nonexistent")
	if ok {
		t.Error("Get(nonexistent) should return false")
	}
}

func TestCollectAll(t *testing.T) {
	reg := collector.NewRegistry()
	reg.Register(&fakeCollector{name: "cpu", cpuVal: 55.5})
	reg.Register(&fakeCollector{name: "mem", memVal: 70.2})

	snap := &model.Snapshot{}
	errs := reg.CollectAll(context.Background(), snap)
	if len(errs) != 0 {
		t.Fatalf("CollectAll() errors: %v", errs)
	}
	if snap.CPU.UsagePercent != 55.5 {
		t.Errorf("CPU.UsagePercent = %v, want 55.5", snap.CPU.UsagePercent)
	}
	if snap.Memory.UsedPercent != 70.2 {
		t.Errorf("Memory.UsedPercent = %v, want 70.2", snap.Memory.UsedPercent)
	}
}

func TestCollectAllWithError(t *testing.T) {
	reg := collector.NewRegistry()
	reg.Register(&fakeCollector{name: "ok"})
	reg.Register(&errorCollector{name: "bad"})

	snap := &model.Snapshot{}
	errs := reg.CollectAll(context.Background(), snap)
	if len(errs) != 1 {
		t.Fatalf("CollectAll() got %d errors, want 1", len(errs))
	}
}

func TestDiskCollector(t *testing.T) {
	// statfs 在 macOS 和 Linux 上都能用于 /
	dc := collector.NewDiskCollector([]string{"/"})
	if dc.Name() != "disk" {
		t.Errorf("Name() = %q, want %q", dc.Name(), "disk")
	}

	snap := &model.Snapshot{}
	err := dc.Collect(context.Background(), snap)
	if err != nil {
		t.Fatalf("Collect() error: %v", err)
	}
	if len(snap.Disks) != 1 {
		t.Fatalf("len(Disks) = %d, want 1", len(snap.Disks))
	}
	if snap.Disks[0].Mount != "/" {
		t.Errorf("Disks[0].Mount = %q, want %q", snap.Disks[0].Mount, "/")
	}
	if snap.Disks[0].TotalBytes == 0 {
		t.Error("Disks[0].TotalBytes should be > 0")
	}
	if snap.Disks[0].UsedPercent <= 0 || snap.Disks[0].UsedPercent > 100 {
		t.Errorf("Disks[0].UsedPercent = %v, out of range", snap.Disks[0].UsedPercent)
	}
}

// --- test helpers ---

type fakeCollector struct {
	name   string
	cpuVal float64
	memVal float64
}

func (f *fakeCollector) Name() string { return f.name }

func (f *fakeCollector) Collect(_ context.Context, snap *model.Snapshot) error {
	if f.cpuVal != 0 {
		snap.CPU.UsagePercent = f.cpuVal
	}
	if f.memVal != 0 {
		snap.Memory.UsedPercent = f.memVal
	}
	return nil
}

type errorCollector struct{ name string }

func (e *errorCollector) Name() string { return e.name }
func (e *errorCollector) Collect(_ context.Context, _ *model.Snapshot) error {
	return context.DeadlineExceeded
}
