package telegram

import (
	"context"

	tgBot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (b *Bot) Send(ctx context.Context, chatID int64, text string) (*models.Message, error) {
	return b.api.SendMessage(ctx, &tgBot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
}
