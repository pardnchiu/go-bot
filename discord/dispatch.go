package discord

import (
	"log/slog"

	"github.com/bwmarrin/discordgo"
)

func (b *Bot) channelName(channelID string) string {
	if channelID == "" || b.api == nil {
		return ""
	}
	if ch, err := b.api.State.Channel(channelID); err == nil && ch != nil {
		return ch.Name
	}
	return ""
}

func (b *Bot) dispatch(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m == nil || m.Message == nil {
		return
	}
	if s.State != nil && s.State.User != nil && m.Author != nil && m.Author.ID == s.State.User.ID {
		return
	}

	b.mu.Lock()
	running := b.running
	ctx := b.ctx
	b.mu.Unlock()
	if !running || ctx == nil {
		return
	}

	var userID, username string
	if m.Author != nil {
		userID = m.Author.ID
		username = m.Author.Username
	}

	b.handlerMu.RLock()
	handler := b.handler
	b.handlerMu.RUnlock()
	if handler == nil {
		return
	}

	reply := replyHandler(ctx, handler, Input{
		ChannelID:   m.ChannelID,
		ChannelName: b.channelName(m.ChannelID),
		GuildID:     m.GuildID,
		MessageID:   m.ID,
		UserID:      userID,
		Username:    username,
		Text:        m.Content,
		Attachments: m.Attachments,
		Raw:         m,
	})
	if reply == "" {
		return
	}

	if _, err := b.api.ChannelMessageSendReply(m.ChannelID, reply, &discordgo.MessageReference{
		MessageID: m.ID,
		ChannelID: m.ChannelID,
		GuildID:   m.GuildID,
	}, discordgo.WithContext(ctx)); err != nil {
		slog.Warn("bwmarrin/discordgo Session.ChannelMessageSendReply",
			slog.String("channelId", m.ChannelID),
			slog.String("err", err.Error()))
	}
}
