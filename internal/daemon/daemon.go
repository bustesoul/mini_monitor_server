package daemon

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"mini_monitor_server/internal/alert"
	"mini_monitor_server/internal/collector"
	"mini_monitor_server/internal/config"
	"mini_monitor_server/internal/httpapi"
	"mini_monitor_server/internal/integration"
	"mini_monitor_server/internal/metrics"
	"mini_monitor_server/internal/model"
	"mini_monitor_server/internal/notifier"
	"mini_monitor_server/internal/rule"
	"mini_monitor_server/internal/storage"
	"mini_monitor_server/internal/telegram"
)

type Daemon struct {
	cfg        *config.Config
	store      *storage.Storage
	collectors *collector.Registry
	netCol     *collector.NetworkCollector
	engine     *rule.Engine
	alertMgr   *alert.Manager
	notifiers  *notifier.Registry
	httpSrv    *httpapi.Server
	integrator *integration.Manager
	tgBot      *telegram.Bot
	state      *model.ServiceState
	accum      *metricsAccumulator
	dirSizer   *storageDirSizer
	mu         sync.RWMutex
}

func New(cfg *config.Config) (*Daemon, error) {
	store, err := storage.New(cfg.Storage.Dir)
	if err != nil {
		return nil, err
	}
	rules := buildRuntimeRules(cfg)

	d := &Daemon{
		cfg:        cfg,
		store:      store,
		collectors: collector.NewRegistry(),
		notifiers:  notifier.NewRegistry(),
		engine:     rule.NewEngine(rules),
		accum:      newMetricsAccumulator(store),
		integrator: integration.NewManager(cfg),
		dirSizer:   newStorageDirSizer(cfg.Storage.Dir, cfg.Storage.DirSizeCheckInterval.Duration),
	}

	// 注册采集器
	d.collectors.Register(collector.NewCPUCollector(cfg.Collector.CPUSampleWindow.Duration))
	d.collectors.Register(collector.NewMemoryCollector())
	if cfg.Disk.Enabled && len(cfg.Disk.Mounts) > 0 {
		d.collectors.Register(collector.NewDiskCollector(cfg.Disk.Mounts))
	}
	if cfg.Network.Enabled && len(cfg.Network.Interfaces) > 0 {
		d.netCol = collector.NewNetworkCollector(cfg.Network.Interfaces)
		d.collectors.Register(d.netCol)
	}

	// 注册通知器
	d.notifiers.Register(notifier.NewLogNotifier())
	if cfg.Notify.Telegram.Enabled {
		bot, err := tgbotapi.NewBotAPI(cfg.Notify.Telegram.BotToken)
		if err != nil {
			return nil, err
		}
		chatID, err := strconv.ParseInt(cfg.Notify.Telegram.ChatID, 10, 64)
		if err != nil {
			return nil, err
		}
		d.notifiers.Register(notifier.NewTelegramNotifier(bot, chatID))

		// Telegram Bot 命令
		if cfg.Notify.Telegram.CommandsEnabled {
			d.tgBot = telegram.NewBot(bot, cfg, d.getSnapshot, d.getMetricsAvg, d.engine, store)
		}
	}

	d.alertMgr = alert.NewManager(
		cfg.Alerts.DedupWindow.Duration,
		cfg.Alerts.RepeatInterval.Duration,
		d.notifiers,
		store,
	)

	d.httpSrv = httpapi.NewServer(cfg.Server.Listen, d.getSnapshot, d.getMetricsAvg, d.engine, store, cfg.History.DefaultDays)

	return d, nil
}

func buildRuntimeRules(cfg *config.Config) []config.RuleConfig {
	rules := append([]config.RuleConfig(nil), cfg.Rules...)
	if cfg.Storage.DirSizeAlertMB > 0 {
		forDuration := cfg.Collector.Interval
		if forDuration.Duration <= 0 {
			forDuration.Duration = time.Minute
		}
		rules = append(rules, config.RuleConfig{
			Name:      config.StorageDirAlertRuleName,
			Type:      "storage_dir_size_mb",
			Threshold: float64(cfg.Storage.DirSizeAlertMB),
			For:       forDuration,
			Severity:  "warning",
		})
	}
	return rules
}

func (d *Daemon) Run(ctx context.Context) error {
	// 加载或初始化状态
	if err := d.initState(); err != nil {
		return err
	}
	d.performStartupMaintenance()

	slog.Info("daemon starting",
		"collectors", d.collectors.List(),
		"interval", d.cfg.Collector.Interval.Duration)

	// 启动 HTTP server
	if err := d.httpSrv.Start(); err != nil {
		return fmt.Errorf("start http server: %w", err)
	}
	d.integrator.Start(ctx)

	// 启动 Telegram Bot
	if d.tgBot != nil {
		go d.tgBot.Start(ctx)
	}

	ticker := time.NewTicker(d.cfg.Collector.Interval.Duration)
	defer ticker.Stop()

	// 每日快照定时器
	dailyCh := d.setupDailyTimer()

	// 首次采集
	d.collectAndEvaluate(ctx)

	for {
		select {
		case <-ctx.Done():
			slog.Info("daemon shutting down")
			d.shutdown()
			return nil
		case <-ticker.C:
			d.collectAndEvaluate(ctx)
		case <-dailyCh:
			d.dailySnapshot()
			dailyCh = d.setupDailyTimer()
		}
	}
}

func (d *Daemon) initState() error {
	saved, err := d.store.LoadState()
	if err != nil {
		slog.Warn("load state failed, starting fresh", "error", err)
	}

	now := time.Now()
	if saved != nil && !d.cfg.Network.ResetBaselineOnStart {
		d.state = saved
		d.engine.RestoreStates(saved.Rules)
		d.alertMgr.RestoreLastSent(saved.Rules)
		if d.netCol != nil && saved.NetworkBaseline != nil {
			d.netCol.SetBaseline(saved.NetworkBaseline)
		}
		slog.Info("state restored", "started_at", saved.StartedAt)
	} else {
		d.state = &model.ServiceState{
			StartedAt:       now,
			NetworkBaseline: make(map[string]model.NetworkBaseline),
			Rules:           make(map[string]model.RuleRuntimeState),
		}
		if d.netCol != nil {
			if err := d.netCol.InitBaseline(); err != nil {
				slog.Warn("init network baseline failed", "error", err)
			} else {
				d.state.NetworkBaseline = d.netCol.GetBaseline()
			}
		}
		slog.Info("fresh state initialized")
	}
	return nil
}

func (d *Daemon) collectAndEvaluate(ctx context.Context) {
	hostname, _ := os.Hostname()
	now := time.Now()

	snap := &model.Snapshot{
		Timestamp: now,
		Hostname:  hostname,
	}

	errs := d.collectors.CollectAll(ctx, snap)
	for _, err := range errs {
		slog.Warn("collector error", "error", err)
	}
	snap.StorageDirSizeBytes = d.dirSizer.Size(now)

	// 规则评估
	events := d.engine.Evaluate(snap, now)
	d.alertMgr.Process(ctx, events)

	// 累加 CPU/Memory 到分钟累加器
	d.accum.Record(snap.Timestamp, snap.CPU.UsagePercent, snap.Memory.UsedPercent)

	// 检查重复提醒
	d.alertMgr.CheckRepeat(ctx, d.engine.FiringRules())

	states := d.engine.States()
	mergeLastSent(states, d.alertMgr.LastSentSnapshot())

	d.mu.Lock()
	d.state.LastSnapshot = snap
	d.state.Rules = states
	if d.netCol != nil {
		d.state.NetworkBaseline = d.netCol.GetBaseline()
	}
	d.mu.Unlock()

	if err := d.store.SaveState(d.state); err != nil {
		slog.Error("save state failed", "error", err)
	}
}

func (d *Daemon) getSnapshot() *model.Snapshot {
	d.mu.RLock()
	defer d.mu.RUnlock()
	if d.state != nil {
		return d.state.LastSnapshot
	}
	return nil
}

func (d *Daemon) getMetricsAvg(base time.Time, windows []int) model.MetricsAvg {
	return metrics.ComputeAverages(d.store, windows, base)
}

func (d *Daemon) setupDailyTimer() <-chan time.Time {
	now := time.Now()
	hour, minute := 0, 0
	if d.cfg.Disk.SampleDailyAt != "" {
		parsed, err := time.Parse("15:04", d.cfg.Disk.SampleDailyAt)
		if err == nil {
			hour = parsed.Hour()
			minute = parsed.Minute()
		}
	}
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, now.Location())
	if next.Before(now) {
		next = next.Add(24 * time.Hour)
	}
	return time.After(time.Until(next))
}

func (d *Daemon) performStartupMaintenance() {
	d.store.CleanHistory(d.cfg.Storage.KeepDaysLocal)
}

func (d *Daemon) dailySnapshot() {
	slog.Info("taking daily snapshot")
	d.mu.RLock()
	snap := d.state.LastSnapshot
	d.mu.RUnlock()

	if snap == nil {
		return
	}

	now := time.Now()
	for _, disk := range snap.Disks {
		if err := d.store.AppendDiskHistory(disk, now); err != nil {
			slog.Error("append disk history failed", "error", err)
		}
	}

	if d.cfg.Network.DailyRollup {
		for _, net := range snap.Networks {
			if err := d.store.AppendNetHistory(net, now); err != nil {
				slog.Error("append net history failed", "error", err)
			}
		}
	}

	// 清理过期历史
	d.store.CleanHistory(d.cfg.Storage.KeepDaysLocal)
}

// CollectOnce 执行一次采集并返回快照（供 CLI report 使用）
func (d *Daemon) CollectOnce(ctx context.Context) (*model.Snapshot, error) {
	hostname, _ := os.Hostname()
	snap := &model.Snapshot{
		Timestamp: time.Now(),
		Hostname:  hostname,
	}
	if d.netCol != nil {
		d.netCol.InitBaseline()
	}
	errs := d.collectors.CollectAll(ctx, snap)
	for _, err := range errs {
		slog.Warn("collector error", "error", err)
	}
	return snap, nil
}

func (d *Daemon) shutdown() {
	d.accum.Flush()
	d.integrator.Stop()
	d.httpSrv.Stop()
	if d.tgBot != nil {
		d.tgBot.Stop()
	}
	d.notifiers.CloseAll()

	states := d.engine.States()
	mergeLastSent(states, d.alertMgr.LastSentSnapshot())

	d.mu.Lock()
	d.state.Rules = states
	if d.netCol != nil {
		d.state.NetworkBaseline = d.netCol.GetBaseline()
	}
	d.mu.Unlock()

	if err := d.store.SaveState(d.state); err != nil {
		slog.Error("save state on shutdown failed", "error", err)
	}
	slog.Info("daemon stopped")
}

func mergeLastSent(states map[string]model.RuleRuntimeState, lastSent map[string]time.Time) {
	for name, state := range states {
		if ts, ok := lastSent[name]; ok {
			copyTs := ts
			state.LastSentAt = &copyTs
		} else {
			state.LastSentAt = nil
		}
		states[name] = state
	}
}

func CollectSnapshot(ctx context.Context, cfg *config.Config) (*model.Snapshot, error) {
	hostname, _ := os.Hostname()
	snap := &model.Snapshot{
		Timestamp: time.Now(),
		Hostname:  hostname,
	}

	collectors := collector.NewRegistry()
	collectors.Register(collector.NewCPUCollector(cfg.Collector.CPUSampleWindow.Duration))
	collectors.Register(collector.NewMemoryCollector())
	if cfg.Disk.Enabled && len(cfg.Disk.Mounts) > 0 {
		collectors.Register(collector.NewDiskCollector(cfg.Disk.Mounts))
	}
	if cfg.Network.Enabled && len(cfg.Network.Interfaces) > 0 {
		netCollector := collector.NewNetworkCollector(cfg.Network.Interfaces)
		if err := netCollector.InitBaseline(); err != nil {
			slog.Warn("init network baseline failed", "error", err)
		} else {
			collectors.Register(netCollector)
		}
	}

	errs := collectors.CollectAll(ctx, snap)
	for _, err := range errs {
		slog.Warn("collector error", "error", err)
	}

	return snap, nil
}
