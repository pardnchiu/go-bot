package discord

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/bwmarrin/discordgo"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"
)

const (
	selectMenuPrefix = "go-bot:select:menu:"
	selectItemMax    = 25
)

type selectState struct {
	channelID       string
	replyTo         string
	promptMessageID string
	multi           bool
}

func (b *Bot) SendSelect(ctx context.Context, channelID, replyTo, text string, items []string) (*discordgo.Message, error) {
	if channelID == "" {
		return nil, fmt.Errorf("channelID is required")
	}
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}

	options := make([]discordgo.SelectMenuOption, 0, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		options = append(options, discordgo.SelectMenuOption{
			Label: item,
			Value: item,
		})
	}
	if len(options) == 0 {
		return nil, fmt.Errorf("items at least one non-empty item is required")
	}
	if len(options) > selectItemMax {
		return nil, fmt.Errorf("items must be 1-%d (got %d)", selectItemMax, len(options))
	}

	uuid := go_pkg_utils.UUID()
	if uuid == "" {
		return nil, fmt.Errorf("github.com/pardnchiu/go-pkg/utils UUID returned empty (crypto/rand failure)")
	}

	minOne := 1
	data := &discordgo.MessageSend{
		Content: text,
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						MenuType:    discordgo.StringSelectMenu,
						CustomID:    selectMenuPrefix + uuid,
						Placeholder: "Select (single-select)",
						MinValues:   &minOne,
						MaxValues:   1,
						Options:     options,
					},
				},
			},
		},
	}
	if replyTo != "" {
		data.Reference = &discordgo.MessageReference{
			MessageID: replyTo,
			ChannelID: channelID,
		}
	}
	msg, err := b.api.ChannelMessageSendComplex(channelID, data, discordgo.WithContext(ctx))
	if err != nil {
		return nil, fmt.Errorf("bwmarrin/discordgo Session.ChannelMessageSendComplex: %w", err)
	}

	b.selectsMu.Lock()
	b.selects[uuid] = &selectState{
		channelID:       channelID,
		replyTo:         replyTo,
		promptMessageID: msg.ID,
	}
	b.selectsMu.Unlock()
	return msg, nil
}

func (b *Bot) handleSelectMenu(i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()
	if !strings.HasPrefix(data.CustomID, selectMenuPrefix) {
		return
	}
	uuid := strings.TrimPrefix(data.CustomID, selectMenuPrefix)

	b.selectsMu.Lock()
	state, ok := b.selects[uuid]
	if ok {
		delete(b.selects, uuid)
	}
	b.selectsMu.Unlock()
	if !ok {
		_ = b.api.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "Expired",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	if err := b.api.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	}); err != nil {
		slog.Warn("bwmarrin/discordgo Session.InteractionRespond (select ack)",
			slog.String("err", err.Error()))
	}

	b.mu.Lock()
	ctx := b.ctx
	running := b.running
	b.mu.Unlock()
	if !running || ctx == nil {
		return
	}
	b.handlerMu.RLock()
	handler := b.handler
	b.handlerMu.RUnlock()
	if handler == nil {
		return
	}

	var userID, username string
	if i.Member != nil && i.Member.User != nil {
		userID = i.Member.User.ID
		username = i.Member.User.Username
	} else if i.User != nil {
		userID = i.User.ID
		username = i.User.Username
	}

	in := Input{
		ChannelID:   state.channelID,
		ChannelName: b.channelName(state.channelID),
		GuildID:     i.GuildID,
		MessageID:   state.promptMessageID,
		UserID:      userID,
		Username:    username,
	}
	if state.multi {
		in.CallbackPicks = data.Values
	} else if len(data.Values) > 0 {
		in.Text = data.Values[0]
	}
	reply := replyHandler(ctx, handler, in)
	if reply == "" {
		return
	}

	target := state.replyTo
	if target == "" {
		target = state.promptMessageID
	}
	if _, err := b.api.ChannelMessageSendReply(state.channelID, reply, &discordgo.MessageReference{
		MessageID: target,
		ChannelID: state.channelID,
	}, discordgo.WithContext(ctx)); err != nil {
		slog.Warn("bwmarrin/discordgo Session.ChannelMessageSendReply (after select)",
			slog.String("channelId", state.channelID),
			slog.String("err", err.Error()))
	}
}
