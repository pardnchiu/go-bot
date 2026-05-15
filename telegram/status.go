package telegram

import (
	"context"
	"strings"
	"time"

	tgBot "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

const (
	statusMinInterval = time.Second
	statusPrefix      = "🤔 "
)

type ChatStatus struct {
	messageID  int
	replyTo    int
	pending    string
	hasPending bool
	lastText   string
	lastFlush  time.Time
	timer      *time.Timer
	inflight   bool
	ctx        context.Context
	finishing  bool
	finishCtx  context.Context
	finishDone chan struct{}
}

func (b *Bot) SendStatus(ctx context.Context, chatID int64, replyTo int, text string) error {
	b.statusMu.Lock()
	status, ok := b.statuses[chatID]
	if !ok {
		status = &ChatStatus{}
		b.statuses[chatID] = status
	}
	if status.messageID == 0 {
		status.replyTo = replyTo
	}
	status.pending = text
	status.hasPending = true
	status.ctx = ctx

	if status.inflight {
		b.statusMu.Unlock()
		return nil
	}

	elapsed := time.Since(status.lastFlush)
	if elapsed >= statusMinInterval {
		b.statusMu.Unlock()
		return b.flushStatus(chatID)
	}

	if status.timer == nil {
		cid := chatID
		status.timer = time.AfterFunc(statusMinInterval-elapsed, func() {
			_ = b.flushStatus(cid)
		})
	}
	b.statusMu.Unlock()
	return nil
}

func (b *Bot) FinishStatus(ctx context.Context, chatID int64) error {
	b.statusMu.Lock()
	status, ok := b.statuses[chatID]
	if !ok {
		b.statusMu.Unlock()
		return nil
	}
	if status.timer != nil {
		status.timer.Stop()
		status.timer = nil
	}

	if status.inflight {
		status.finishing = true
		status.finishCtx = ctx
		status.finishDone = make(chan struct{})

		done := status.finishDone
		b.statusMu.Unlock()
		select {
		case <-done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	msgID := status.messageID
	delete(b.statuses, chatID)
	b.statusMu.Unlock()

	return b.doFinishChat(ctx, chatID, msgID)
}

func (b *Bot) flushStatus(chatID int64) error {
	b.statusMu.Lock()
	status, ok := b.statuses[chatID]
	if !ok {
		b.statusMu.Unlock()
		return nil
	}
	if status.inflight || !status.hasPending {
		status.timer = nil
		b.statusMu.Unlock()
		return nil
	}

	text := status.pending
	status.pending = ""
	status.hasPending = false
	status.timer = nil
	status.inflight = true

	msgID := status.messageID
	replyTo := status.replyTo
	ctx := status.ctx
	decorated := statusPrefix + text
	skip := msgID != 0 && decorated == status.lastText
	b.statusMu.Unlock()

	if skip {
		b.afterFlushStatus(chatID, status, msgID, decorated, nil)
		return nil
	}

	var newID int
	var err error
	if msgID == 0 {
		params := &tgBot.SendMessageParams{
			ChatID: chatID,
			Text:   decorated,
		}
		if replyTo != 0 {
			params.ReplyParameters = &models.ReplyParameters{MessageID: replyTo}
		}
		m, sendErr := b.api.SendMessage(ctx, params)
		err = sendErr
		if m != nil {
			newID = m.ID
		}
	} else {
		_, editErr := b.api.EditMessageText(ctx, &tgBot.EditMessageTextParams{
			ChatID:    chatID,
			MessageID: msgID,
			Text:      decorated,
		})
		err = editErr
		newID = msgID
		if err != nil && strings.Contains(err.Error(), "message is not modified") {
			err = nil
		}
	}
	b.afterFlushStatus(chatID, status, newID, decorated, err)
	return err
}

func (b *Bot) afterFlushStatus(chatID int64, status *ChatStatus, newID int, newText string, _ error) {
	b.statusMu.Lock()
	current, ok := b.statuses[chatID]
	if !ok || current != status {
		b.statusMu.Unlock()
		return
	}
	status.inflight = false
	status.lastFlush = time.Now()
	if status.messageID == 0 && newID != 0 {
		status.messageID = newID
	}
	status.lastText = newText

	if status.finishing {
		finishCtx := status.finishCtx
		msgID := status.messageID
		done := status.finishDone
		delete(b.statuses, chatID)
		b.statusMu.Unlock()

		_ = b.doFinishChat(finishCtx, chatID, msgID)
		close(done)
		return
	}

	if status.hasPending && status.timer == nil {
		delay := statusMinInterval - time.Since(status.lastFlush)
		if delay <= 0 {
			delay = time.Millisecond
		}
		cid := chatID
		status.timer = time.AfterFunc(delay, func() {
			_ = b.flushStatus(cid)
		})
	}
	b.statusMu.Unlock()
}

func (b *Bot) doFinishChat(ctx context.Context, chatID int64, msgID int) error {
	if msgID == 0 {
		return nil
	}
	_, err := b.api.DeleteMessage(ctx, &tgBot.DeleteMessageParams{
		ChatID:    chatID,
		MessageID: msgID,
	})
	return err
}
