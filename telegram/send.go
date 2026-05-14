package telegram

import (
	"context"

	tgBot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Input struct {
	ChatID   int64
	UserID   int64
	Username string
	Text     string
	Raw      *models.Update
}

type ReplyHandler func(ctx context.Context, input Input) string

func (b *Bot) Reply(handler ReplyHandler) {
	b.handlerMu.Lock()
	b.handler = handler
	b.handlerMu.Unlock()
}

func replyHandler(ctx context.Context, handler ReplyHandler, input Input) (out string) {
	defer func() {
		if r := recover(); r != nil {
			out = r.(string)
		}
	}()
	return handler(ctx, input)
}

func (b *Bot) Send(ctx context.Context, chatID int64, text string) (*models.Message, error) {
	return b.api.SendMessage(ctx, &tgBot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	})
}
