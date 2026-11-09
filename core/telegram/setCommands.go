package telegram

import (
	"context"
	"fmt"

	tgBot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Command struct {
	Name        string
	Description string
}

func (b *Bot) SetCommands(ctx context.Context, commands []Command) error {
	list := make([]models.BotCommand, 0, len(commands))
	for _, c := range commands {
		if c.Name == "" || c.Description == "" {
			return fmt.Errorf("[Invalid] command: name and description required")
		}
		list = append(list, models.BotCommand{Command: c.Name, Description: c.Description})
	}
	if _, err := b.api.SetMyCommands(ctx, &tgBot.SetMyCommandsParams{Commands: list}); err != nil {
		return fmt.Errorf("[Upstream] setMyCommands: %w", err)
	}
	return nil
}
