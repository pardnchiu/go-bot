package telegram

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	tgBot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Input struct {
	ChatID   int64
	UserID   int64
	Username string
	Text     string
	Caption  string
	Photo    []models.PhotoSize
	Document *models.Document
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

type FileType int

const (
	TypePhoto FileType = iota
	TypeDocument
)

func (b *Bot) SendFile(ctx context.Context, chatID int64, fileType FileType, path string, caption ...string) (*models.Message, error) {
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("os.Open: %w", err)
	}
	defer file.Close()

	upload := &models.InputFileUpload{
		Filename: filepath.Base(path),
		Data:     file,
	}
	var captionStr string
	if len(caption) > 0 {
		captionStr = caption[0]
	}

	switch fileType {
	case TypePhoto:
		return b.api.SendPhoto(ctx, &tgBot.SendPhotoParams{
			ChatID:  chatID,
			Photo:   upload,
			Caption: captionStr,
		})
	case TypeDocument:
		return b.api.SendDocument(ctx, &tgBot.SendDocumentParams{
			ChatID:   chatID,
			Document: upload,
			Caption:  captionStr,
		})
	default:
		return nil, fmt.Errorf("unknown file type: %d", fileType)
	}
}
