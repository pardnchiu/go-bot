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
	inputButtonPrefix = "go-bot:input:btn:"
	inputModalPrefix  = "go-bot:input:modal:"
	inputTextID       = "go-bot:input:text"
	modalTitleMax     = 45
)

type inputState struct {
	channelID       string
	replyTo         string
	promptMessageID string
	prompt          string
}

func (b *Bot) SendInput(ctx context.Context, channelID, replyTo, prompt string) (*discordgo.Message, error) {
	if channelID == "" {
		return nil, fmt.Errorf("channelID is required")
	}
	if prompt == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	uuid := go_pkg_utils.UUID()
	if uuid == "" {
		return nil, fmt.Errorf("github.com/pardnchiu/go-pkg/utils UUID returned empty (crypto/rand failure)")
	}

	data := &discordgo.MessageSend{
		Content: prompt,
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.Button{
						Label:    "回答",
						Style:    discordgo.PrimaryButton,
						CustomID: inputButtonPrefix + uuid,
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

	b.inputsMu.Lock()
	b.inputs[uuid] = &inputState{
		channelID:       channelID,
		replyTo:         replyTo,
		promptMessageID: msg.ID,
		prompt:          prompt,
	}
	b.inputsMu.Unlock()
	return msg, nil
}

func (b *Bot) interactionDispatch(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i == nil || i.Interaction == nil {
		return
	}
	switch i.Type {
	case discordgo.InteractionMessageComponent:
		data := i.MessageComponentData()
		switch {
		case strings.HasPrefix(data.CustomID, inputButtonPrefix):
			b.handleInputButton(i)
		case strings.HasPrefix(data.CustomID, selectMenuPrefix):
			b.handleSelectMenu(i)
		}
	case discordgo.InteractionModalSubmit:
		b.handleInputModalSubmit(i)
	}
}

func (b *Bot) handleInputButton(i *discordgo.InteractionCreate) {
	data := i.MessageComponentData()
	if !strings.HasPrefix(data.CustomID, inputButtonPrefix) {
		return
	}
	uuid := strings.TrimPrefix(data.CustomID, inputButtonPrefix)

	b.inputsMu.Lock()
	state, ok := b.inputs[uuid]
	b.inputsMu.Unlock()
	if !ok {
		_ = b.api.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "已過期，請重新觸發",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	title := state.prompt
	if len(title) > modalTitleMax {
		title = title[:modalTitleMax]
	}

	err := b.api.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseModal,
		Data: &discordgo.InteractionResponseData{
			CustomID: inputModalPrefix + uuid,
			Title:    title,
			Components: []discordgo.MessageComponent{
				discordgo.ActionsRow{
					Components: []discordgo.MessageComponent{
						discordgo.TextInput{
							CustomID: inputTextID,
							Label:    "回答",
							Style:    discordgo.TextInputShort,
							Required: true,
						},
					},
				},
			},
		},
	})
	if err != nil {
		slog.Warn("bwmarrin/discordgo Session.InteractionRespond (modal)",
			slog.String("err", err.Error()))
	}
}

func (b *Bot) handleInputModalSubmit(i *discordgo.InteractionCreate) {
	data := i.ModalSubmitData()
	if !strings.HasPrefix(data.CustomID, inputModalPrefix) {
		return
	}
	uuid := strings.TrimPrefix(data.CustomID, inputModalPrefix)

	b.inputsMu.Lock()
	state, ok := b.inputs[uuid]
	if ok {
		delete(b.inputs, uuid)
	}
	b.inputsMu.Unlock()
	if !ok {
		_ = b.api.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseChannelMessageWithSource,
			Data: &discordgo.InteractionResponseData{
				Content: "已過期，請重新觸發",
				Flags:   discordgo.MessageFlagsEphemeral,
			},
		})
		return
	}

	var text string
	for _, row := range data.Components {
		ar, ok := row.(*discordgo.ActionsRow)
		if !ok {
			continue
		}
		for _, c := range ar.Components {
			ti, ok := c.(*discordgo.TextInput)
			if !ok {
				continue
			}
			if ti.CustomID == inputTextID {
				text = ti.Value
			}
		}
	}

	if err := b.api.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: "✓",
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	}); err != nil {
		slog.Warn("bwmarrin/discordgo Session.InteractionRespond (modal ack)",
			slog.String("err", err.Error()))
	}
	if err := b.api.InteractionResponseDelete(i.Interaction); err != nil {
		slog.Warn("bwmarrin/discordgo Session.InteractionResponseDelete (ephemeral ack cleanup)",
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

	reply := replyHandler(ctx, handler, Input{
		ChannelID: state.channelID,
		GuildID:   i.GuildID,
		MessageID: state.promptMessageID,
		UserID:    userID,
		Username:  username,
		Text:      text,
	})
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
		slog.Warn("bwmarrin/discordgo Session.ChannelMessageSendReply (after modal)",
			slog.String("channelId", state.channelID),
			slog.String("err", err.Error()))
	}
}
