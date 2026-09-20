package discord

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) SendVoice(ctx context.Context, channelID, replyTo, path string, caption ...string) (*discordgo.Message, error) {
	if channelID == "" {
		return nil, fmt.Errorf("channelID is required")
	}
	if path == "" {
		return nil, fmt.Errorf("path is required")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("os.Open: %w", err)
	}
	defer file.Close()

	data := &discordgo.MessageSend{
		Files: []*discordgo.File{{
			Name:        filepath.Base(path),
			ContentType: "audio/ogg",
			Reader:      file,
		}},
	}
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
