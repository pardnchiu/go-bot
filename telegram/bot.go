package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"sync"

	tgBot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type Status struct {
	Running  bool
	Username string
	UserID   int64
}

type Bot struct {
	api           *tgBot.Bot
	mu            sync.Mutex
	cancel        context.CancelFunc
	done          chan struct{}
	running       bool
	me            *models.User
	handlerMu     sync.RWMutex
	handler       ReplyHandler
	statusMu      sync.Mutex
	statuses      map[int64]*ChatStatus
	multiSelectMu sync.Mutex
	multiSelects  map[multiSelectKey]*multiSelectState
}

func New(token string) (*Bot, error) {
	if token == "" {
		return nil, fmt.Errorf("token is required")
	}

	bot := &Bot{
		statuses:     make(map[int64]*ChatStatus),
		multiSelects: make(map[multiSelectKey]*multiSelectState),
	}
	api, err := tgBot.New(token,
		tgBot.WithDefaultHandler(bot.dispatch),
		tgBot.WithErrorsHandler(func(err error) {
			slog.Warn("Bot.New",
				slog.String("err", err.Error()))
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("Bot.New: %w", err)
	}
	bot.api = api
	return bot, nil
}

func (b *Bot) Start(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.running {
		return nil
	}

	me, err := b.api.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("go-telegram/bot Bot.GetMe: %w", err)
	}
	info, err := b.api.GetWebhookInfo(ctx)
	switch {
	case err != nil:
		slog.Warn("go-telegram/bot Bot.GetWebhookInfo",
			slog.String("err", err.Error()))
	case info.URL != "":
		if _, err := b.api.DeleteWebhook(ctx, &tgBot.DeleteWebhookParams{DropPendingUpdates: false}); err != nil {
			slog.Warn("go-telegram/bot Bot.DeleteWebhook",
				slog.String("err", err.Error()))
		}
	}

	runCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	b.cancel = cancel
	b.done = done
	b.running = true
	b.me = me

	go func() {
		defer close(done)
		b.api.Start(runCtx)
	}()
	return nil
}

func (b *Bot) Close() error {
	b.mu.Lock()
	if !b.running {
		b.mu.Unlock()
		return nil
	}

	cancel := b.cancel
	done := b.done
	b.running = false
	b.cancel = nil
	b.done = nil
	b.mu.Unlock()

	cancel()
	<-done

	b.statusMu.Lock()
	for _, e := range b.statuses {
		if e.timer != nil {
			e.timer.Stop()
			e.timer = nil
		}
	}
	b.statuses = make(map[int64]*ChatStatus)
	b.statusMu.Unlock()

	b.multiSelectMu.Lock()
	b.multiSelects = make(map[multiSelectKey]*multiSelectState)
	b.multiSelectMu.Unlock()
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
