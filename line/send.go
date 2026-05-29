package line

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/line/line-bot-sdk-go/v8/linebot"
)

type Input struct {
	SourceType string
	UserID     string
	GroupID    string
	RoomID     string
	ReplyToken string
	MessageID  string
	Text       string
	Raw        *linebot.Event
}

type ReplyHandler func(ctx context.Context, input Input) string

func (b *Bot) Reply(handler ReplyHandler) {
	b.handlerMu.Lock()
	b.handler = handler
	b.handlerMu.Unlock()
}

func replyHandler(ctx context.Context, handler ReplyHandler, input Input) (out string) {
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		slog.Warn("reply handler panic",
			slog.String("userId", input.UserID),
			slog.Any("recover", r))
		if s, ok := r.(string); ok {
			out = s
		} else {
			out = ""
		}
	}()
	return handler(ctx, input)
}

func (b *Bot) Send(ctx context.Context, to, text string) (*linebot.BasicResponse, error) {
	if to == "" {
		return nil, fmt.Errorf("to is required")
	}
	return b.api.PushMessage(to, linebot.NewTextMessage(text)).WithContext(ctx).Do()
}
