package telegram

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	tgBot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (b *Bot) SendPhoto(ctx context.Context, chatID int64, paths []string, caption ...string) ([]*models.Message, error) {
	newPaths := []string{}
	for _, path := range paths {
		if path != "" {
			newPaths = append(newPaths, path)
		}
	}

	if len(newPaths) == 0 {
		return nil, fmt.Errorf("paths at least one path is required")
	} else if len(newPaths) > 10 {
		return nil, fmt.Errorf("paths at most 10 paths are allowed")
	}

	var captionStr string
	if len(caption) > 0 {
		captionStr = caption[0]
	}

	if len(newPaths) == 1 {
		file, err := os.Open(newPaths[0])
		if err != nil {
			return nil, fmt.Errorf("os.Open: %w", err)
		}
		defer file.Close()

		msg, err := b.api.SendPhoto(ctx, &tgBot.SendPhotoParams{
			ChatID: chatID,
			Photo: &models.InputFileUpload{
				Filename: filepath.Base(newPaths[0]),
				Data:     file,
			},
			Caption: captionStr,
		})
		if err != nil {
			return nil, err
		}
		return []*models.Message{msg}, nil
	}

	media := make([]models.InputMedia, 0, len(newPaths))
	files := make([]*os.File, 0, len(newPaths))
	defer func() {
		for _, f := range files {
			_ = f.Close()
		}
	}()

	for i, path := range newPaths {
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("os.Open: %w", err)
		}

		files = append(files, file)
		photo := &models.InputMediaPhoto{
			Media:           fmt.Sprintf("attach://m%d", i),
			MediaAttachment: file,
		}
		if i == 0 {
			photo.Caption = captionStr
		}
		media = append(media, photo)
	}

	return b.api.SendMediaGroup(ctx, &tgBot.SendMediaGroupParams{
		ChatID: chatID,
		Media:  media,
	})
}
