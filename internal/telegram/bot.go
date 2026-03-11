package telegram

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"mini_monitor_server/internal/config"
	"mini_monitor_server/internal/model"
	"mini_monitor_server/internal/rule"
	"mini_monitor_server/internal/storage"
	"mini_monitor_server/internal/telegram/command"
)

type Bot struct {
	bot            *tgbotapi.BotAPI
	cfg            *config.Config
	allowedChatIDs map[int64]bool
	commands       *command.Registry
	stopFn         context.CancelFunc
}

func NewBot(bot *tgbotapi.BotAPI, cfg *config.Config, getSnapshot func() *model.Snapshot, getMetricsAvg func(time.Time, []int) model.MetricsAvg, engine *rule.Engine, store *storage.Storage) *Bot {
	allowed := make(map[int64]bool)
	for _, id := range cfg.Notify.Telegram.AllowedChatIDs {
		if n, err := strconv.ParseInt(id, 10, 64); err == nil {
			allowed[n] = true
		}
	}

	reg := command.NewRegistry()
	command.RegisterAll(reg, getSnapshot, getMetricsAvg, engine, store, cfg)

	return &Bot{
		bot:            bot,
		cfg:            cfg,
		allowedChatIDs: allowed,
		commands:       reg,
	}
}

func (b *Bot) Start(ctx context.Context) {
	ctx, b.stopFn = context.WithCancel(ctx)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 30
	updates := b.bot.GetUpdatesChan(u)

	slog.Info("telegram bot started", "username", b.bot.Self.UserName)

	for {
		select {
		case <-ctx.Done():
			return
		case update := <-updates:
			if update.Message == nil || !update.Message.IsCommand() {
				continue
			}
			if !b.allowedChatIDs[update.Message.Chat.ID] {
				slog.Debug("telegram: ignored unauthorized chat", "chat_id", update.Message.Chat.ID)
				continue
			}
			b.handleCommand(ctx, update.Message)
		}
	}
}

func (b *Bot) Stop() {
	if b.stopFn != nil {
		b.stopFn()
	}
	b.bot.StopReceivingUpdates()
}

func (b *Bot) handleCommand(ctx context.Context, msg *tgbotapi.Message) {
	cmdName := strings.ToLower(msg.Command())
	args := msg.CommandArguments()

	cmd, ok := b.commands.Get(cmdName)
	if !ok {
		reply := tgbotapi.NewMessage(msg.Chat.ID, "Unknown command. Use /help to see available commands.")
		b.bot.Send(reply)
		return
	}

	result, err := cmd.Execute(ctx, args)
	if err != nil {
		slog.Error("telegram command error", "command", cmdName, "error", err)
		result = "Error: " + err.Error()
	}

	reply := tgbotapi.NewMessage(msg.Chat.ID, result)
	if _, err := b.bot.Send(reply); err != nil {
		slog.Error("telegram send failed", "error", err)
	}
}
