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

func (b *Bot) displayName(ctx context.Context, src *linebot.EventSource) string {
	if src == nil || src.UserID == "" {
		return ""
	}
	var res *linebot.UserProfileResponse
	var err error
	switch src.Type {
	case linebot.EventSourceTypeGroup:
		res, err = b.api.GetGroupMemberProfile(src.GroupID, src.UserID).WithContext(ctx).Do()
	case linebot.EventSourceTypeRoom:
		res, err = b.api.GetRoomMemberProfile(src.RoomID, src.UserID).WithContext(ctx).Do()
	default:
		res, err = b.api.GetProfile(src.UserID).WithContext(ctx).Do()
	}
	if err != nil {
		slog.Warn("line-bot-sdk-go GetProfile",
			slog.String("userId", src.UserID),
			slog.String("sourceType", string(src.Type)),
			slog.String("err", err.Error()))
		return ""
	}
	return res.DisplayName
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
		Username:   b.displayName(ctx, event.Source),
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
