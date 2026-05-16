package telegram

import (
	"context"
	"fmt"

	tgBot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (b *Bot) SendInput(ctx context.Context, chatID int64, replyTo int, text string, sendType ...SendType) (*models.Message, error) {
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}
	params := &tgBot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: models.ForceReply{ForceReply: true},
	}
	if replyTo > 0 {
		params.ReplyParameters = &models.ReplyParameters{MessageID: replyTo}
	}
	if len(sendType) > 0 {
		switch sendType[0] {
		case TypeMarkdown:
			params.ParseMode = models.ParseModeMarkdown
		case TypeHTML:
			params.ParseMode = models.ParseModeHTML
		}
	}
	return b.api.SendMessage(ctx, params)
}
