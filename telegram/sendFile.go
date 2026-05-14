package telegram

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	tgBot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type FileType int

const (
	TypePhoto FileType = iota
	TypeDocument
	TypeVideo
	TypeAudio
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
	case TypeVideo:
		return b.api.SendVideo(ctx, &tgBot.SendVideoParams{
			ChatID:  chatID,
			Video:   upload,
			Caption: captionStr,
		})
	case TypeAudio:
		return b.api.SendAudio(ctx, &tgBot.SendAudioParams{
			ChatID:  chatID,
			Audio:   upload,
			Caption: captionStr,
		})
	default:
		return nil, fmt.Errorf("unknown file type: %d", fileType)
	}
}
