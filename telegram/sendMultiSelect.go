package telegram

import (
	"context"
	"fmt"
	"log/slog"

	tgBot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const multiSelectDoneData = "__bot_multiselect_done__"

type multiSelectKey struct {
	chatID int64
	msgID  int
}

type multiSelectState struct {
	items    []string
	selected map[string]bool
	doneData string
}

func (b *Bot) SendMultiSelect(ctx context.Context, chatID int64, replyTo int, text string, items []string) (*models.Message, error) {
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

	state := &multiSelectState{
		items:    cleanItems,
		selected: make(map[string]bool, len(cleanItems)),
		doneData: multiSelectDoneData,
	}

	params := &tgBot.SendMessageParams{
		ChatID:      chatID,
		Text:        text,
		ReplyMarkup: buildMultiSelectMarkup(state),
	}
	if replyTo > 0 {
		params.ReplyParameters = &models.ReplyParameters{MessageID: replyTo}
	}
	msg, err := b.api.SendMessage(ctx, params)
	if err != nil {
		return nil, err
	}

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

	if query.Data == state.doneData {
		b.multiSelectMu.Lock()
		picks := make([]string, 0, len(state.items))
		for _, item := range state.items {
			if state.selected[item] {
				picks = append(picks, item)
			}
		}
		delete(b.multiSelects, multiSelectKey{chatID: promptMsg.Chat.ID, msgID: promptMsg.ID})
		b.multiSelectMu.Unlock()

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

	b.multiSelectMu.Lock()
	state.selected[query.Data] = !state.selected[query.Data]
	newMarkup := buildMultiSelectMarkup(state)
	b.multiSelectMu.Unlock()

	if _, err := b.api.EditMessageReplyMarkup(ctx, &tgBot.EditMessageReplyMarkupParams{
		ChatID:      promptMsg.Chat.ID,
		MessageID:   promptMsg.ID,
		ReplyMarkup: newMarkup,
	}); err != nil {
		slog.Warn("go-telegram/bot Bot.EditMessageReplyMarkup",
			slog.String("err", err.Error()))
	}
}
