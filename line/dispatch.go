package line

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/line/line-bot-sdk-go/v8/linebot"
)

func (b *Bot) webhook(w http.ResponseWriter, r *http.Request) {
	events, err := b.api.ParseRequest(r)
	if err != nil {
		if err == linebot.ErrInvalidSignature {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		slog.Warn("line-bot-sdk-go ParseRequest",
			slog.String("err", err.Error()))
		return
	}

	b.mu.Lock()
	running := b.running
	ctx := b.ctx
	b.mu.Unlock()
	if !running || ctx == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	reqCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	for _, event := range events {
		b.handleEvent(reqCtx, event)
	}
	w.WriteHeader(http.StatusOK)
}

func (b *Bot) handleEvent(ctx context.Context, event *linebot.Event) {
	if event.Type != linebot.EventTypeMessage {
		return
	}
	msg, ok := event.Message.(*linebot.TextMessage)
	if !ok {
		return
	}

	b.handlerMu.RLock()
	handler := b.handler
	b.handlerMu.RUnlock()
	if handler == nil {
		slog.Warn("Reply is not set",
			slog.String("userId", event.Source.UserID))
		return
	}

	reply := replyHandler(ctx, handler, Input{
		SourceType: string(event.Source.Type),
		UserID:     event.Source.UserID,
		GroupID:    event.Source.GroupID,
		RoomID:     event.Source.RoomID,
		ReplyToken: event.ReplyToken,
		MessageID:  msg.ID,
		Text:       msg.Text,
		Raw:        event,
	})
	if reply == "" {
		return
	}

	if _, err := b.api.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(reply)).WithContext(ctx).Do(); err != nil {
		slog.Warn("line-bot-sdk-go ReplyMessage",
			slog.String("userId", event.Source.UserID),
			slog.String("err", err.Error()))
	}
}
