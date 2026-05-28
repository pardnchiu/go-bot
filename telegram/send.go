package telegram

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"

	tgBot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"github.com/pardnchiu/go-bot/tts"
)

type SendType int

const (
	TypeMarkdown SendType = iota
	TypeHTML
)

type MessageOption func(*messageOptions)

type messageOptions struct {
	parseMode models.ParseMode
}

func WithSendType(t SendType) MessageOption {
	return func(o *messageOptions) {
		o.parseMode = parseMode(t)
	}
}

func parseMode(t SendType) models.ParseMode {
	switch t {
	case TypeMarkdown:
		return models.ParseModeMarkdown
	case TypeHTML:
		return models.ParseModeHTML
	}
	return ""
}

type Input struct {
	ChatID        int64
	ChatName      string
	MessageID     int
	UserID        int64
	Username      string
	Text          string
	Caption       string
	Photo         []models.PhotoSize
	Document      *models.Document
	CallbackData  string
	CallbackPicks []string
	Raw           *models.Update
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
			slog.Int64("chatId", input.ChatID),
			slog.Any("recover", r))
		if s, ok := r.(string); ok {
			out = s
		} else {
			out = ""
		}
	}()
	return handler(ctx, input)
}

func (b *Bot) Send(ctx context.Context, chatID int64, replyTo int, text string, opts ...MessageOption) (*models.Message, error) {
	mo := messageOptions{}
	for _, opt := range opts {
		opt(&mo)
	}
	params := &tgBot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: mo.parseMode,
	}
	if replyTo > 0 {
		params.ReplyParameters = &models.ReplyParameters{MessageID: replyTo}
	}
	return b.api.SendMessage(ctx, params)
}

func (b *Bot) Delete(ctx context.Context, chatID int64, msgID int) error {
	_, err := b.api.DeleteMessage(ctx, &tgBot.DeleteMessageParams{
		ChatID:    chatID,
		MessageID: msgID,
	})
	return err
}

func (b *Bot) SendVoice(ctx context.Context, chatID int64, text, apiKey string, caption ...string) (*models.Message, error) {
	ogg, err := tts.Get(ctx, apiKey, text)
	if err != nil {
		return nil, fmt.Errorf("github.com/pardnchiu/go-bot/tts Get: %w", err)
	}
	params := &tgBot.SendVoiceParams{
		ChatID: chatID,
		Voice: &models.InputFileUpload{
			Filename: "voice.ogg",
			Data:     bytes.NewReader(ogg),
		},
	}
	if len(caption) > 0 {
		params.Caption = caption[0]
	}
	return b.api.SendVoice(ctx, params)
}
