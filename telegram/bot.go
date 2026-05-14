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
	api     *tgBot.Bot
	mu      sync.Mutex
	cancel  context.CancelFunc
	done    chan struct{}
	running bool
	me      *models.User

	handlerMu sync.RWMutex
	handler   ReplyHandler
}

func New(token string) (*Bot, error) {
	if token == "" {
		return nil, fmt.Errorf("token is required")
	}

	bot := &Bot{}
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
	if _, err := b.api.DeleteWebhook(ctx, &tgBot.DeleteWebhookParams{DropPendingUpdates: false}); err != nil {
		slog.Warn("go-telegram/bot Bot.DeleteWebhook",
			slog.String("err", err.Error()))
	}

	runCtx, cancel := context.WithCancel(ctx)
	b.cancel = cancel
	b.done = make(chan struct{})
	b.running = true
	b.me = me

	go func() {
		defer close(b.done)
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

func (b *Bot) dispatch(ctx context.Context, _ *tgBot.Bot, update *models.Update) {
	if update == nil || update.Message == nil {
		return
	}

	msg := update.Message
	var userID int64
	var username string
	if msg.From != nil {
		userID = msg.From.ID
		username = msg.From.Username
	}
	slog.Info("telegram update",
		slog.Int64("chatId", msg.Chat.ID),
		slog.String("text", msg.Text))

	b.handlerMu.RLock()
	handler := b.handler
	b.handlerMu.RUnlock()
	if handler == nil {
		slog.Warn("Reply is not set",
			slog.Int64("chatId", msg.Chat.ID))
		return
	}

	reply := replyHandler(ctx, handler, Input{
		ChatID:   msg.Chat.ID,
		UserID:   userID,
		Username: username,
		Text:     msg.Text,
		Raw:      update,
	})
	if reply == "" {
		return
	}

	if _, err := b.api.SendMessage(ctx, &tgBot.SendMessageParams{
		ChatID: msg.Chat.ID,
		Text:   reply,
	}); err != nil {
		slog.Warn("go-telegram/bot Bot.SendMessage",
			slog.Int64("chatId", msg.Chat.ID),
			slog.String("err", err.Error()))
	}
}
