// Telegram-адаптер: пересылка сообщений в buh-ai-agent и подтверждение операций.
package bot

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot"

	"github.com/leavion/buh-ai-tg-adapter/internal/agentapi"
)

// Bot — Telegram-адаптер к агенту.
type Bot struct {
	tb      *bot.Bot
	agent   *agentapi.Client
	log     *slog.Logger
	allowed map[int64]struct{}
}

// New создаёт адаптер и регистрирует обработчики.
func New(tb *bot.Bot, agent *agentapi.Client, log *slog.Logger, allowed []int64) *Bot {
	b := &Bot{
		tb:      tb,
		agent:   agent,
		log:     log,
		allowed: make(map[int64]struct{}, len(allowed)),
	}
	for _, id := range allowed {
		b.allowed[id] = struct{}{}
	}

	b.tb.RegisterHandler(bot.HandlerTypeMessageText, "/start", bot.MatchTypeCommand, b.handleStart)
	b.tb.RegisterHandler(bot.HandlerTypeMessageText, "/help", bot.MatchTypeCommand, b.handleStart)
	// Все остальные текстовые сообщения пересылаем агенту.
	b.tb.RegisterHandler(bot.HandlerTypeMessageText, "", bot.MatchTypePrefix, b.handleMessage)
	b.tb.RegisterHandler(bot.HandlerTypeCallbackQueryData, cbPrefix, bot.MatchTypePrefix, b.handleCallback)

	return b
}

// Start запускает long polling (блокирует до отмены контекста).
func (b *Bot) Start(ctx context.Context) {
	b.tb.Start(ctx)
}

// allowedUser проверяет, разрешён ли пользователь.
func (b *Bot) allowedUser(id int64) bool {
	if len(b.allowed) == 0 {
		return true
	}
	_, ok := b.allowed[id]
	return ok
}
