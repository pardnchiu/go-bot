package discord

import (
	"context"
	"fmt"
	"sync"

	"github.com/bwmarrin/discordgo"
)

type Status struct {
	Running  bool
	Username string
	UserID   string
}

type Bot struct {
	api       *discordgo.Session
	mu        sync.Mutex
	cancel    context.CancelFunc
	ctx       context.Context
	running   bool
	me        *discordgo.User
	handlerMu sync.RWMutex
	handler   ReplyHandler
}

func New(token string) (*Bot, error) {
	if token == "" {
		return nil, fmt.Errorf("token is required")
	}

	api, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, fmt.Errorf("bwmarrin/discordgo New: %w", err)
	}
	api.Identify.Intents = discordgo.IntentsGuildMessages | discordgo.IntentDirectMessages | discordgo.IntentMessageContent

	bot := &Bot{api: api}
	api.AddHandler(bot.dispatch)
	return bot, nil
}

func (b *Bot) Start(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.running {
		return nil
	}

	me, err := b.api.User("@me")
	if err != nil {
		return fmt.Errorf("bwmarrin/discordgo Session.User(@me): %w", err)
	}
	if err := b.api.Open(); err != nil {
		return fmt.Errorf("bwmarrin/discordgo Session.Open: %w", err)
	}

	runCtx, cancel := context.WithCancel(ctx)
	b.ctx = runCtx
	b.cancel = cancel
	b.running = true
	b.me = me
	return nil
}

func (b *Bot) Close() error {
	b.mu.Lock()
	if !b.running {
		b.mu.Unlock()
		return nil
	}
	cancel := b.cancel
	b.cancel = nil
	b.ctx = nil
	b.running = false
	b.mu.Unlock()

	cancel()
	if err := b.api.Close(); err != nil {
		return fmt.Errorf("bwmarrin/discordgo Session.Close: %w", err)
	}
	return nil
}

func (b *Bot) Status() Status {
	b.mu.Lock()
	defer b.mu.Unlock()

	status := Status{Running: b.running}
	if b.me != nil {
		status.Username = b.me.Username
		status.UserID = b.me.ID
	}
	return status
}
