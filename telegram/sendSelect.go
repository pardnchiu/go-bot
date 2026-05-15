package telegram

import (
	"context"
	"fmt"

	tgBot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (b *Bot) SendSelect(ctx context.Context, chatID int64, replyTo int, text string, items []string) (*models.Message, error) {
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}

	keyboard := make([][]models.InlineKeyboardButton, 0, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		keyboard = append(keyboard, []models.InlineKeyboardButton{{
			Text:         item,
			CallbackData: item,
		}})
	}
	if len(keyboard) == 0 {
		return nil, fmt.Errorf("items at least one non-empty item is required")
	}

	params := &tgBot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
		ReplyMarkup: models.InlineKeyboardMarkup{
			InlineKeyboard: keyboard,
		},
	}
	if replyTo > 0 {
		params.ReplyParameters = &models.ReplyParameters{MessageID: replyTo}
	}
	return b.api.SendMessage(ctx, params)
}
