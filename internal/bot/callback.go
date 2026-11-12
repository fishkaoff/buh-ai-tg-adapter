// Обработка нажатий кнопок подтверждения/отклонения операции.
package bot

import (
	"context"
	"strconv"
	"strings"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/leavion/buh-ai-tg-adapter/internal/agentapi"
)

// handleCallback обрабатывает нажатие кнопок «Подтвердить»/«Отклонить».
func (b *Bot) handleCallback(ctx context.Context, _ *bot.Bot, update *models.Update) {
	cq := update.CallbackQuery
	if cq == nil || cq.Message.Message == nil {
		return
	}
	msg := cq.Message.Message

	// Формат callback_data: op:<confirm|reject>:<operation_id-uuid>.
	parts := strings.SplitN(cq.Data, ":", 3)
	if len(parts) != 3 || parts[0] != cbPrefix {
		return
	}
	action, operationID := parts[1], parts[2]

	chatID := msg.Chat.ID
	userID := cq.From.ID
	if !b.allowedUser(userID) {
		b.log.Warn("нажатие кнопки запрещённым пользователем", "user_id", userID, "chat_id", chatID)
		b.answerCallback(ctx, cq.ID, "⛔️ Доступ к агенту запрещён.", true)
		return
	}

	b.log.Info("нажатие кнопки", "action", action, "operation_id", operationID, "chat_id", chatID, "user_id", userID)

	var (
		resp *agentapi.StatusResponse
		err  error
	)
	switch action {
	case cbAction:
		resp, err = b.agent.ConfirmOperation(ctx, operationID, strconv.FormatInt(chatID, 10))
	case cbActionReject:
		resp, err = b.agent.RejectOperation(ctx, operationID, strconv.FormatInt(chatID, 10))
	default:
		b.answerCallback(ctx, cq.ID, "Неизвестное действие.", true)
		return
	}

	if err != nil {
		b.log.Error("агент не принял команду", "action", action, "operation_id", operationID, "err", err)
		switch {
		case agentapi.ErrNotFound(err):
			b.answerCallback(ctx, cq.ID, "Операция не найдена.", true)
			b.editStatus(ctx, msg, "⚠️ Операция не найдена.")
		case agentapi.ErrForbidden(err):
			// Операция принадлежит другому чату — не показываем детали.
			b.answerCallback(ctx, cq.ID, "⛔️ Недоступно.", true)
		case agentapi.ErrConflict(err):
			b.answerCallback(ctx, cq.ID, "Операция уже обработана.", true)
			b.editStatus(ctx, msg, "ℹ️ Операция уже обработана.")
		default:
			b.answerCallback(ctx, cq.ID, "⚠️ Сервис агента недоступен. Попробуйте позже.", true)
		}
		return
	}

	b.log.Info("статус операции изменён", "operation_id", resp.OperationID, "status", resp.Status)

	b.answerCallback(ctx, cq.ID, "", false)
	// Убираем клавиатуру и фиксируем результат в исходном сообщении.
	b.editStatus(ctx, msg, statusLine(action, resp.Status))
}

// answerCallback закрывает индикатор загрузки на кнопке.
func (b *Bot) answerCallback(ctx context.Context, callbackID, text string, showAlert bool) {
	_, err := b.tb.AnswerCallbackQuery(ctx, &bot.AnswerCallbackQueryParams{
		CallbackQueryID: callbackID,
		Text:            text,
		ShowAlert:       showAlert,
	})
	if err != nil {
		b.log.Error("не удалось ответить на callback", "err", err)
	}
}

// editStatus убирает клавиатуру и дописывает статус к исходному сообщению.
func (b *Bot) editStatus(ctx context.Context, msg *models.Message, status string) {
	text := msg.Text
	if text == "" {
		return
	}
	_, err := b.tb.EditMessageText(ctx, &bot.EditMessageTextParams{
		ChatID:      msg.Chat.ID,
		MessageID:   msg.ID,
		Text:        text + "\n\n" + status,
		ReplyMarkup: models.ReplyMarkup(nil),
	})
	if err != nil {
		b.log.Error("не удалось обновить сообщение", "chat_id", msg.Chat.ID, "err", err)
	}
}
