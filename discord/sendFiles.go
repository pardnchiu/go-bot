package discord

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) SendFiles(ctx context.Context, channelID, replyTo string, paths []string, caption ...string) (*discordgo.Message, error) {
	if channelID == "" {
		return nil, fmt.Errorf("channelID is required")
	}
	if len(paths) == 0 || len(paths) > 10 {
		return nil, fmt.Errorf("paths must be 1-10 (got %d)", len(paths))
	}

	files := make([]*discordgo.File, 0, len(paths))
	defer func() {
		for _, f := range files {
			if c, ok := f.Reader.(*os.File); ok {
				c.Close()
			}
		}
	}()
	for _, p := range paths {
		if p == "" {
			return nil, fmt.Errorf("path is required")
		}
		f, err := os.Open(p)
		if err != nil {
			return nil, fmt.Errorf("os.Open: %w", err)
		}
		files = append(files, &discordgo.File{
			Name:   filepath.Base(p),
			Reader: f,
		})
	}

	data := &discordgo.MessageSend{Files: files}
	if len(caption) > 0 {
		data.Content = caption[0]
	}
	if replyTo != "" {
		data.Reference = &discordgo.MessageReference{
			MessageID: replyTo,
			ChannelID: channelID,
		}
	}
	return b.api.ChannelMessageSendComplex(channelID, data, discordgo.WithContext(ctx))
}
