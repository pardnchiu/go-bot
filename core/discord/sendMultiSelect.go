package discord

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
	go_pkg_utils "github.com/pardnchiu/go-pkg/utils"
)

func (b *Bot) SendMultiSelect(ctx context.Context, channelID, replyTo, text string, items []string) (*discordgo.Message, error) {
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

	minZero := 0
	data := &discordgo.MessageSend{
		Content: text,
		Components: []discordgo.MessageComponent{
			discordgo.ActionsRow{
				Components: []discordgo.MessageComponent{
					discordgo.SelectMenu{
						MenuType:    discordgo.StringSelectMenu,
						CustomID:    selectMenuPrefix + uuid,
						Placeholder: "Select (multi-select)",
						MinValues:   &minZero,
						MaxValues:   len(options),
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
		multi:           true,
	}
	b.selectsMu.Unlock()
	return msg, nil
}
