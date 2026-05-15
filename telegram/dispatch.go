package telegram

import (
	"context"
	"log/slog"

	tgBot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func (b *Bot) dispatch(ctx context.Context, _ *tgBot.Bot, update *models.Update) {
	if update == nil {
		return
	}
	if update.CallbackQuery != nil {
		b.dispatchCallback(ctx, update)
		return
	}
	if update.Message == nil {
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
		slog.String("text", msg.Text),
		slog.String("caption", msg.Caption),
		slog.Int("photoCount", len(msg.Photo)),
		slog.Bool("hasDocument", msg.Document != nil))

	b.handlerMu.RLock()
	handler := b.handler
	b.handlerMu.RUnlock()
	if handler == nil {
		slog.Warn("Reply is not set",
			slog.Int64("chatId", msg.Chat.ID))
		return
	}

	reply := replyHandler(ctx, handler, Input{
		ChatID:    msg.Chat.ID,
		MessageID: msg.ID,
		UserID:    userID,
		Username:  username,
		Text:      msg.Text,
		Caption:   msg.Caption,
		Photo:     msg.Photo,
		Document:  msg.Document,
		Raw:       update,
	})
	if reply == "" {
		return
	}

	if _, err := b.api.SendMessage(ctx, &tgBot.SendMessageParams{
		ChatID:          msg.Chat.ID,
		Text:            reply,
		ReplyParameters: &models.ReplyParameters{MessageID: msg.ID},
	}); err != nil {
		slog.Warn("go-telegram/bot Bot.SendMessage",
			slog.Int64("chatId", msg.Chat.ID),
			slog.String("err", err.Error()))
	}
}

func (b *Bot) dispatchCallback(ctx context.Context, update *models.Update) {
	query := update.CallbackQuery
	if query.Message.Type != models.MaybeInaccessibleMessageTypeMessage || query.Message.Message == nil {
		return
	}
	promptMsg := query.Message.Message

	if _, err := b.api.AnswerCallbackQuery(ctx, &tgBot.AnswerCallbackQueryParams{
		CallbackQueryID: query.ID,
	}); err != nil {
		slog.Warn("go-telegram/bot Bot.AnswerCallbackQuery",
			slog.String("err", err.Error()))
	}

	b.multiSelectMu.Lock()
	msState, isMulti := b.multiSelects[multiSelectKey{chatID: promptMsg.Chat.ID, msgID: promptMsg.ID}]
	b.multiSelectMu.Unlock()
	if isMulti {
		b.handleMultiSelectCallback(ctx, update, msState)
		return
	}

	if _, err := b.api.EditMessageReplyMarkup(ctx, &tgBot.EditMessageReplyMarkupParams{
		ChatID:      promptMsg.Chat.ID,
		MessageID:   promptMsg.ID,
		ReplyMarkup: models.InlineKeyboardMarkup{InlineKeyboard: [][]models.InlineKeyboardButton{}},
	}); err != nil {
		slog.Warn("go-telegram/bot Bot.EditMessageReplyMarkup",
			slog.String("err", err.Error()))
	}

	slog.Info("telegram callback",
		slog.Int64("chatId", promptMsg.Chat.ID),
		slog.String("data", query.Data),
		slog.Int("promptMsgId", promptMsg.ID))

	b.handlerMu.RLock()
	handler := b.handler
	b.handlerMu.RUnlock()
	if handler == nil {
		slog.Warn("Reply is not set",
			slog.Int64("chatId", promptMsg.Chat.ID))
		return
	}

	reply := replyHandler(ctx, handler, Input{
		ChatID:       promptMsg.Chat.ID,
		MessageID:    promptMsg.ID,
		UserID:       query.From.ID,
		Username:     query.From.Username,
		CallbackData: query.Data,
		Raw:          update,
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
}
