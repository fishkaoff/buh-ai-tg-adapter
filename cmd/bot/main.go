// Точка входа Telegram-адаптера buh-ai-agent.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-telegram/bot"

	"github.com/leavion/buh-ai-tg-adapter/internal/agentapi"
	telegrambot "github.com/leavion/buh-ai-tg-adapter/internal/bot"
	"github.com/leavion/buh-ai-tg-adapter/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("конфигурация", "err", err)
		os.Exit(1)
	}

	log := newLogger(cfg.LogLevel)

	log.Info("запуск адаптера",
		"agent_base_url", cfg.AgentBaseURL,
		"agent_timeout", cfg.AgentTimeout,
		"allowed_users", len(cfg.AllowedUserIDs),
	)

	// Проверяем доступность агента на старте (не критично — просто логируем).
	agent := agentapi.New(cfg.AgentBaseURL, cfg.AgentTimeout)
	healthCtx, cancelHealth := context.WithTimeout(context.Background(), 5*time.Second)
	if err := agent.Health(healthCtx); err != nil {
		log.Warn("агент недоступен на старте", "err", err)
	} else {
		log.Info("агент доступен")
	}
	cancelHealth()

	tb, err := bot.New(cfg.TelegramBotToken)
	if err != nil {
		log.Error("создание Telegram-бота", "err", err)
		os.Exit(1)
	}

	adapter := telegrambot.New(tb, agent, log, cfg.AllowedUserIDs)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	log.Info("бот запущен, long polling")
	adapter.Start(ctx)
	log.Info("бот остановлен")
}

// newLogger создаёт slog-логгер по уровню из конфига.
func newLogger(level string) *slog.Logger {
	var lv slog.Level
	switch level {
	case "debug":
		lv = slog.LevelDebug
	case "warn":
		lv = slog.LevelWarn
	case "error":
		lv = slog.LevelError
	default:
		lv = slog.LevelInfo
	}
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: lv}))
}
