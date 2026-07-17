package telegram

import (
	"context"
	"fmt"

	tgBot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (b *Bot) SendInput(ctx context.Context, chatID int64, replyTo int, text string, opts ...MessageOption) (*models.Message, error) {
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}
	mo := messageOptions{}
	for _, opt := range opts {
		opt(&mo)
	}
	params := &tgBot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   mo.parseMode,
		ReplyMarkup: models.ForceReply{ForceReply: true},
	}
	if replyTo > 0 {
		params.ReplyParameters = &models.ReplyParameters{MessageID: replyTo}
	}
	return b.api.SendMessage(ctx, params)
}
