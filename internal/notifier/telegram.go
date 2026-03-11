package notifier

import (
	"context"
	"fmt"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"mini_monitor_server/internal/model"
)

// TelegramNotifier Telegram 告警推送
type TelegramNotifier struct {
	bot    *tgbotapi.BotAPI
	chatID int64
}

func NewTelegramNotifier(bot *tgbotapi.BotAPI, chatID int64) *TelegramNotifier {
	return &TelegramNotifier{bot: bot, chatID: chatID}
}

func (t *TelegramNotifier) Name() string { return "telegram" }

func (t *TelegramNotifier) SendAlert(_ context.Context, evt *model.AlertEvent) error {
	text := formatAlert(evt)
	msg := tgbotapi.NewMessage(t.chatID, text)
	_, err := t.bot.Send(msg)
	return err
}

func (t *TelegramNotifier) SendRecovery(_ context.Context, evt *model.AlertEvent) error {
	text := formatRecovery(evt)
	msg := tgbotapi.NewMessage(t.chatID, text)
	_, err := t.bot.Send(msg)
	return err
}

func (t *TelegramNotifier) Close() error { return nil }

func formatAlert(evt *model.AlertEvent) string {
	msg := evt.Message
	if msg == "" {
		msg = fmt.Sprintf("[%s] %s\nvalue: %.1f%%\ntime: %s",
			evt.Severity, evt.Rule, evt.Value,
			evt.Timestamp.UTC().Format("2006-01-02 15:04:05 UTC"))
	}
	return msg
}

func formatRecovery(evt *model.AlertEvent) string {
	return fmt.Sprintf("[RECOVERED] %s\nvalue: %.1f%%\nrecovered_at: %s",
		evt.Rule, evt.Value,
		evt.Timestamp.UTC().Format("2006-01-02 15:04:05 UTC"))
}
