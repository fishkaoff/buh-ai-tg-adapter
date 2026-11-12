// Обработка текстовых сообщений: пересылка в агент.
package bot

import (
	"bytes"
	"context"
	"strconv"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/leavion/buh-ai-tg-adapter/internal/agentapi"
)

// greeting — приветствие для /start и /help.
const greeting = "Я — адаптер к AI-агенту бухгалтерии 1С.\n\n" +
	"Пришлите запрос текстом, например: «Создай счёт на услуги для ООО Ромашка " +
	"на 100 000 рублей, НДС 5%». Когда агент подготовит операцию, я предложу " +
	"её подтвердить или отклонить."

// handleStart отвечает на /start и /help.
func (b *Bot) handleStart(ctx context.Context, _ *bot.Bot, update *models.Update) {
	msg := update.Message
	if msg == nil {
		return
	}
	b.log.Info("пользователь запросил справку", "chat_id", msg.Chat.ID, "user_id", msg.From.ID)

	if _, err := b.tb.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: msg.Chat.ID,
		Text:   greeting,
	}); err != nil {
		b.log.Error("не удалось отправить приветствие", "err", err)
	}
}

// handleMessage пересылает текстовое сообщение агенту и отправляет ответ.
func (b *Bot) handleMessage(ctx context.Context, _ *bot.Bot, update *models.Update) {
	msg := update.Message
	if msg == nil || msg.Text == "" {
		return
	}

	chatID, userID := msg.Chat.ID, msg.From.ID
	if !b.allowedUser(userID) {
		b.log.Warn("запрос от запрещённого пользователя", "user_id", userID, "chat_id", chatID)
		b.sendText(ctx, chatID, "⛔️ Доступ к агенту запрещён.")
		return
	}

	b.log.Info("сообщение пользователя", "chat_id", chatID, "user_id", userID, "len", len(msg.Text))

	// Отправляем «печатает…» — агент может думать долго.
	typingCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	go b.sendTypingWhile(typingCtx, chatID)

	resp, err := b.agent.Chat(ctx, strconv.FormatInt(chatID, 10), strconv.FormatInt(userID, 10), msg.Text)
	if err != nil {
		b.log.Error("агент не ответил", "chat_id", chatID, "err", err)
		b.sendText(ctx, chatID, "⚠️ Сервис агента недоступен. Попробуйте позже.")
		return
	}

	b.log.Info("ответ агента",
		"chat_id", chatID,
		"operation_id", resp.OperationID,
		"operation_status", resp.OperationStatus,
	)

	params := &bot.SendMessageParams{
		ChatID: chatID,
		Text:   resp.Reply,
		ReplyParameters: &models.ReplyParameters{
			MessageID:                msg.ID,
			AllowSendingWithoutReply: true,
		},
	}
	// Кнопки показываем только для операции, ожидающей подтверждения.
	if resp.OperationID != nil && resp.OperationStatus != nil &&
		*resp.OperationID != "" && *resp.OperationStatus == "awaiting_confirmation" {
		params.ReplyMarkup = confirmKeyboard(*resp.OperationID)
	}

	if _, err := b.tb.SendMessage(ctx, params); err != nil {
		b.log.Error("не удалось отправить ответ", "chat_id", chatID, "err", err)
	}

	// Файлы (PDF печатных форм) отправляем вслед за текстом ответа.
	b.sendResponseFiles(ctx, msg.ID, chatID, resp.Files)
}

// sendResponseFiles отправляет файлы из ответа агента документами в Telegram.
func (b *Bot) sendResponseFiles(ctx context.Context, replyTo int, chatID int64, files []agentapi.ResponseFile) {
	const maxFileBytes = 50 << 20 // лимит Telegram для документов бота

	for _, f := range files {
		data, err := f.Data()
		if err != nil {
			b.log.Error("не удалось декодировать файл агента", "name", f.Name, "err", err)
			b.sendText(ctx, chatID, "⚠️ Не удалось получить файл "+f.Name+".")
			continue
		}
		if len(data) > maxFileBytes {
			b.log.Warn("файл слишком большой для Telegram", "name", f.Name, "bytes", len(data))
			b.sendText(ctx, chatID, "⚠️ Файл "+f.Name+" слишком большой для отправки ("+
				strconv.Itoa(len(data))+" байт).")
			continue
		}

		name := f.Name
		if name == "" {
			name = "document.pdf"
		}

		if _, err := b.tb.SendDocument(ctx, &bot.SendDocumentParams{
			ChatID:   chatID,
			Document: &models.InputFileUpload{Filename: name, Data: bytes.NewReader(data)},
			ReplyParameters: &models.ReplyParameters{
				MessageID:                replyTo,
				AllowSendingWithoutReply: true,
			},
		}); err != nil {
			b.log.Error("не удалось отправить файл", "name", name, "chat_id", chatID, "err", err)
		} else {
			b.log.Info("файл отправлен", "name", name, "chat_id", chatID, "bytes", len(data))
		}
	}
}

// sendTypingWhile отправляет chat action «печатает…» до отмены контекста.
func (b *Bot) sendTypingWhile(ctx context.Context, chatID int64) {
	for {
		_, err := b.tb.SendChatAction(ctx, &bot.SendChatActionParams{
			ChatID: chatID,
			Action: models.ChatActionTyping,
		})
		if err != nil {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(time.Second):
		}
	}
}

// sendText отправляет сообщение без клавиатуры.
func (b *Bot) sendText(ctx context.Context, chatID int64, text string) {
	if _, err := b.tb.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text}); err != nil {
		b.log.Error("не удалось отправить сообщение", "chat_id", chatID, "err", err)
	}
}
