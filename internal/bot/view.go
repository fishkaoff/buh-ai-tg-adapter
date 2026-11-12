// Вспомогательные функции: клавиатуры, форматирование статусов.
package bot

import (
	"github.com/go-telegram/bot/models"
)

// Префиксы callback_data. Идентификатор операции — UUID (36 символов),
// вместе с префиксом укладывается в лимит Telegram 64 байта.
const (
	cbPrefix       = "op:"
	cbAction       = "confirm"
	cbActionReject = "reject"
)

// confirmKeyboard — inline-клавиатура подтверждения/отклонения операции.
func confirmKeyboard(operationID string) models.InlineKeyboardMarkup {
	return models.InlineKeyboardMarkup{
		InlineKeyboard: [][]models.InlineKeyboardButton{
			{
				{Text: "✅ Подтвердить", CallbackData: cbPrefix + cbAction + ":" + operationID},
				{Text: "❌ Отклонить", CallbackData: cbPrefix + cbActionReject + ":" + operationID},
			},
		},
	}
}

// statusLine — строка статуса после подтверждения/отклонения.
func statusLine(action string, status string) string {
	switch {
	case action == cbAction:
		return "✅ Операция подтверждена"
	case action == cbActionReject:
		return "❌ Операция отклонена"
	default:
		return "ℹ️ Статус операции: " + status
	}
}
