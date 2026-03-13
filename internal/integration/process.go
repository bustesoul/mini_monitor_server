package integration

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

type processSpec struct {
	binary string
	args   []string
}

type supervisedProcess struct {
	name string
	spec processSpec

	mu  sync.Mutex
	cmd *exec.Cmd
}

func newSupervisedProcess(name string, spec processSpec) *supervisedProcess {
	return &supervisedProcess{name: name, spec: spec}
}

func (p *supervisedProcess) Start(ctx context.Context) {
	go p.run(ctx)
}

func (p *supervisedProcess) run(ctx context.Context) {
	for {
		cmd := exec.Command(p.spec.binary, p.spec.args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Start(); err != nil {
			slog.Error("start integration process failed", "name", p.name, "binary", p.spec.binary, "error", err)
			return
		}

		p.mu.Lock()
		p.cmd = cmd
		p.mu.Unlock()

		slog.Info("integration process started", "name", p.name, "pid", cmd.Process.Pid)

		waitCh := make(chan error, 1)
		go func() {
			waitCh <- cmd.Wait()
		}()

		select {
		case <-ctx.Done():
			p.stopCurrent(waitCh, 5*time.Second)
			return
		case err := <-waitCh:
			p.clearCmd(cmd)
			if err != nil {
				slog.Warn("integration process exited", "name", p.name, "error", err)
			} else {
				slog.Warn("integration process exited", "name", p.name)
			}
			select {
			case <-ctx.Done():
				return
			case <-time.After(5 * time.Second):
			}
		}
	}
}

func (p *supervisedProcess) Stop(timeout time.Duration) {
	p.mu.Lock()
	cmd := p.cmd
	p.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		slog.Debug("signal integration process failed", "name", p.name, "error", err)
		return
	}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		p.mu.Lock()
		stillRunning := p.cmd == cmd
		p.mu.Unlock()
		if !stillRunning {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		slog.Debug("kill integration process failed", "name", p.name, "error", err)
	}
}

func (p *supervisedProcess) stopCurrent(waitCh <-chan error, timeout time.Duration) {
	p.mu.Lock()
	cmd := p.cmd
	p.mu.Unlock()
	if cmd == nil || cmd.Process == nil {
		return
	}

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		slog.Debug("signal integration process failed", "name", p.name, "error", err)
	}

	select {
	case <-waitCh:
	case <-time.After(timeout):
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			slog.Debug("kill integration process failed", "name", p.name, "error", err)
		}
		<-waitCh
	}

	p.clearCmd(cmd)
}

func (p *supervisedProcess) clearCmd(cmd *exec.Cmd) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cmd == cmd {
		p.cmd = nil
	}
}
