package discord

import (
	"bytes"
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"

	"github.com/pardnchiu/go-bot/core/tts"
)

func (b *Bot) SendVoice(ctx context.Context, channelID, replyTo, text, apiKey string, caption ...string) (*discordgo.Message, error) {
	if channelID == "" {
		return nil, fmt.Errorf("channelID is required")
	}
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}

	ogg, err := tts.Get(ctx, apiKey, text)
	if err != nil {
		return nil, fmt.Errorf("github.com/pardnchiu/go-bot/core/tts Get: %w", err)
	}

	data := &discordgo.MessageSend{
		Files: []*discordgo.File{{
			Name:        "voice.ogg",
			ContentType: "audio/ogg",
			Reader:      bytes.NewReader(ogg),
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
