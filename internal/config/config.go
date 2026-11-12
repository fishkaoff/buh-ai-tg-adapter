// Конфигурация Telegram-адаптера из переменных окружения.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config — настройки адаптера.
type Config struct {
	// TelegramBotToken — токен бота из @BotFather (обязателен).
	TelegramBotToken string
	// AgentBaseURL — базовый URL сервиса buh-ai-agent.
	AgentBaseURL string
	// AgentTimeout — таймаут HTTP-запросов к агенту. /chat может
	// выполняться долго (LLM + до 24 итераций), поэтому дефолт большой.
	AgentTimeout time.Duration
	// AllowedUserIDs — белый список Telegram user_id (пусто — разрешено всем).
	AllowedUserIDs []int64
	// LogLevel — уровень логирования slog (debug|info|warn|error).
	LogLevel string
}

// Load читает конфигурацию из окружения.
func Load() (*Config, error) {
	cfg := &Config{
		AgentBaseURL: "http://127.0.0.1:8000",
		AgentTimeout: 5 * time.Minute,
		LogLevel:     "info",
	}

	cfg.TelegramBotToken = os.Getenv("TELEGRAM_BOT_TOKEN")
	if cfg.TelegramBotToken == "" {
		return nil, errors.New("переменная TELEGRAM_BOT_TOKEN обязательна (токен бота из @BotFather)")
	}

	if v := os.Getenv("AGENT_BASE_URL"); v != "" {
		cfg.AgentBaseURL = v
	}
	cfg.AgentBaseURL = strings.TrimRight(cfg.AgentBaseURL, "/")

	if v := os.Getenv("AGENT_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("некорректная AGENT_TIMEOUT %q: %w", v, err)
		}
		cfg.AgentTimeout = d
	}

	if v := os.Getenv("ALLOWED_USER_IDS"); v != "" {
		for _, part := range strings.Split(v, ",") {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}
			id, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				return nil, fmt.Errorf("некорректный ALLOWED_USER_IDS %q: %w", part, err)
			}
			cfg.AllowedUserIDs = append(cfg.AllowedUserIDs, id)
		}
	}

	if v := os.Getenv("LOG_LEVEL"); v != "" {
		cfg.LogLevel = v
	}

	return cfg, nil
}
