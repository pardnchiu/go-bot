package discord

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
)

type Input struct {
	ChannelID   string
	GuildID     string
	MessageID   string
	UserID      string
	Username    string
	Text        string
	Attachments []*discordgo.MessageAttachment
	Raw         *discordgo.MessageCreate
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
			out = ""
		}
	}()
	return handler(ctx, input)
}

func (b *Bot) Send(ctx context.Context, channelID, replyTo, text string) (*discordgo.Message, error) {
	if channelID == "" {
		return nil, fmt.Errorf("channelID is required")
	}
	data := &discordgo.MessageSend{Content: text}
	if replyTo != "" {
		data.Reference = &discordgo.MessageReference{
			MessageID: replyTo,
			ChannelID: channelID,
		}
	}
	return b.api.ChannelMessageSendComplex(channelID, data, discordgo.WithContext(ctx))
}
