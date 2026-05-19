package telegram

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	tgBot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	multiSelectDoneData    = "__bot_multiselect_done__"
	multiSelectMinInterval = time.Second
	multiSelectStaleTTL    = 5 * time.Minute
)

type multiSelectKey struct {
	chatID int64
	msgID  int
}

type multiSelectState struct {
	items    []string
	selected map[string]bool
	doneData string
	chatID   int64
	msgID    int

	mu        sync.Mutex
	done      bool
	pending   bool
	inflight  bool
	timer     *time.Timer
	lastFlush time.Time
	ctx       context.Context
}

func (b *Bot) SendMultiSelect(ctx context.Context, chatID int64, replyTo int, text string, items []string, opts ...MessageOption) (*models.Message, error) {
	if text == "" {
		return nil, fmt.Errorf("text is required")
	}

	cleanItems := make([]string, 0, len(items))
	for _, item := range items {
		if item == "" {
			continue
		}
		if item == multiSelectDoneData {
			return nil, fmt.Errorf("items must not contain reserved")
		}
		cleanItems = append(cleanItems, item)
	}
	if len(cleanItems) == 0 {
		return nil, fmt.Errorf("items at least one item")
	}

	mo := messageOptions{}
	for _, opt := range opts {
		opt(&mo)
	}

	state := &multiSelectState{
		items:    cleanItems,
		selected: make(map[string]bool, len(cleanItems)),
		doneData: multiSelectDoneData,
		chatID:   chatID,
	}

	params := &tgBot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ParseMode:   mo.parseMode,
		ReplyMarkup: buildMultiSelectMarkup(state),
	}
	if replyTo > 0 {
		params.ReplyParameters = &models.ReplyParameters{MessageID: replyTo}
	}
	msg, err := b.api.SendMessage(ctx, params)
	if err != nil {
		return nil, err
	}

	state.msgID = msg.ID

	b.multiSelectMu.Lock()
	b.multiSelects[multiSelectKey{chatID: chatID, msgID: msg.ID}] = state
	b.multiSelectMu.Unlock()
	return msg, nil
}

func buildMultiSelectMarkup(state *multiSelectState) models.InlineKeyboardMarkup {
	keyboard := make([][]models.InlineKeyboardButton, 0, len(state.items)+1)
	for _, item := range state.items {
		prefix := "⬜ "
		if state.selected[item] {
			prefix = "✅ "
		}
		keyboard = append(keyboard, []models.InlineKeyboardButton{{
			Text:         prefix + item,
			CallbackData: item,
		}})
	}
	keyboard = append(keyboard, []models.InlineKeyboardButton{{
		Text:         "Done",
		CallbackData: state.doneData,
	}})
	return models.InlineKeyboardMarkup{InlineKeyboard: keyboard}
}

func (b *Bot) handleMultiSelectCallback(ctx context.Context, update *models.Update, state *multiSelectState) {
	query := update.CallbackQuery
	promptMsg := query.Message.Message
	key := multiSelectKey{chatID: promptMsg.Chat.ID, msgID: promptMsg.ID}

	state.mu.Lock()
	if state.done {
		state.mu.Unlock()
		return
	}

	if query.Data == state.doneData {
		state.done = true
		if state.timer != nil {
			state.timer.Stop()
			state.timer = nil
		}
		state.pending = false
		picks := make([]string, 0, len(state.items))
		for _, item := range state.items {
			if state.selected[item] {
				picks = append(picks, item)
			}
		}
		state.mu.Unlock()

		time.AfterFunc(multiSelectStaleTTL, func() {
			b.multiSelectMu.Lock()
			delete(b.multiSelects, key)
			b.multiSelectMu.Unlock()
		})

		if _, err := b.api.EditMessageReplyMarkup(ctx, &tgBot.EditMessageReplyMarkupParams{
			ChatID:      promptMsg.Chat.ID,
			MessageID:   promptMsg.ID,
			ReplyMarkup: models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{}},
		}); err != nil {
			slog.Warn("go-telegram/bot Bot.EditMessageReplyMarkup",
				slog.String("err", err.Error()))
		}

		b.handlerMu.RLock()
		handler := b.handler
		b.handlerMu.RUnlock()
		if handler == nil {
			slog.Warn("Reply is not set",
				slog.Int64("chatId", promptMsg.Chat.ID))
			return
		}

		reply := replyHandler(ctx, handler, Input{
			ChatID:        promptMsg.Chat.ID,
			ChatName:      chatName(&promptMsg.Chat),
			MessageID:     promptMsg.ID,
			UserID:        query.From.ID,
			Username:      query.From.Username,
			CallbackPicks: picks,
			Raw:           update,
		})
		if reply == "" {
			return
		}
		if _, err := b.api.SendMessage(ctx, &tgBot.SendMessageParams{
			ChatID:          promptMsg.Chat.ID,
			Text:            reply,
			ReplyParameters: &models.ReplyParameters{MessageID: promptMsg.ID},
		}); err != nil {
			slog.Warn("go-telegram/bot Bot.SendMessage",
				slog.Int64("chatId", promptMsg.Chat.ID),
				slog.String("err", err.Error()))
		}
		return
	}

	state.selected[query.Data] = !state.selected[query.Data]
	state.pending = true
	state.ctx = ctx

	if state.inflight {
		state.mu.Unlock()
		return
	}

	elapsed := time.Since(state.lastFlush)
	if elapsed >= multiSelectMinInterval {
		state.mu.Unlock()
		b.flushMultiSelectEdit(state)
		return
	}
	if state.timer == nil {
		s := state
		state.timer = time.AfterFunc(multiSelectMinInterval-elapsed, func() {
			b.flushMultiSelectEdit(s)
		})
	}
	state.mu.Unlock()
}

func (b *Bot) flushMultiSelectEdit(state *multiSelectState) {
	state.mu.Lock()
	if state.done {
		state.timer = nil
		state.mu.Unlock()
		return
	}
	if state.inflight || !state.pending {
		state.timer = nil
		state.mu.Unlock()
		return
	}
	state.pending = false
	state.timer = nil
	state.inflight = true

	markup := buildMultiSelectMarkup(state)
	chatID := state.chatID
	msgID := state.msgID
	ctx := state.ctx
	state.mu.Unlock()

	if _, err := b.api.EditMessageReplyMarkup(ctx, &tgBot.EditMessageReplyMarkupParams{
		ChatID:      chatID,
		MessageID:   msgID,
		ReplyMarkup: markup,
	}); err != nil {
		slog.Warn("go-telegram/bot Bot.EditMessageReplyMarkup",
			slog.String("err", err.Error()))
	}

	state.mu.Lock()
	state.inflight = false
	state.lastFlush = time.Now()
	if state.done {
		state.mu.Unlock()
		return
	}
	if state.pending && state.timer == nil {
		delay := multiSelectMinInterval - time.Since(state.lastFlush)
		if delay <= 0 {
			delay = time.Millisecond
		}
		s := state
		state.timer = time.AfterFunc(delay, func() {
			b.flushMultiSelectEdit(s)
		})
	}
	state.mu.Unlock()
}
