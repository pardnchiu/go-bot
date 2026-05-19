package discord

import (
	"context"
	"log/slog"
	"time"

	"github.com/bwmarrin/discordgo"
)

const (
	statusMinInterval     = time.Second
	defaultStatusReaction = "🤔"
)

type StatusOption func(*statusOptions)

type statusOptions struct {
	emoji string
}

func WithStatusEmoji(emoji string) StatusOption {
	return func(o *statusOptions) {
		o.emoji = emoji
	}
}

type ChannelStatus struct {
	messageID  string
	replyTo    string
	reaction   string
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

func (b *Bot) SendStatus(ctx context.Context, channelID, replyTo, text string, opts ...StatusOption) error {
	so := statusOptions{}
	for _, opt := range opts {
		opt(&so)
	}

	b.statusMu.Lock()
	status, ok := b.statuses[channelID]
	if !ok {
		status = &ChannelStatus{}
		b.statuses[channelID] = status
	}
	if status.messageID == "" {
		status.replyTo = replyTo
	}
	if status.reaction == "" {
		if so.emoji != "" {
			status.reaction = so.emoji
		} else {
			status.reaction = defaultStatusReaction
		}
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
		return b.flushStatus(channelID)
	}

	if status.timer == nil {
		cid := channelID
		status.timer = time.AfterFunc(statusMinInterval-elapsed, func() {
			_ = b.flushStatus(cid)
		})
	}
	b.statusMu.Unlock()
	return nil
}

func (b *Bot) FinishStatus(ctx context.Context, channelID string) error {
	b.statusMu.Lock()
	status, ok := b.statuses[channelID]
	if !ok {
		b.statusMu.Unlock()
		return nil
	}
	if status.timer != nil {
		status.timer.Stop()
		status.timer = nil
	}

	if status.inflight {
		if status.finishDone == nil {
			status.finishing = true
			status.finishCtx = ctx
			status.finishDone = make(chan struct{})
		}
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
	replyTo := status.replyTo
	reaction := status.reaction
	delete(b.statuses, channelID)
	b.statusMu.Unlock()

	return b.doFinishChat(ctx, channelID, msgID, replyTo, reaction)
}

func (b *Bot) flushStatus(channelID string) error {
	b.statusMu.Lock()
	status, ok := b.statuses[channelID]
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
	reaction := status.reaction
	ctx := status.ctx
	skip := msgID != "" && text == status.lastText
	b.statusMu.Unlock()

	if skip {
		b.afterFlushStatus(channelID, status, msgID, text, nil)
		return nil
	}

	var newID string
	var err error
	if msgID == "" {
		if replyTo != "" {
			if reactErr := b.api.MessageReactionAdd(channelID, replyTo, reaction, discordgo.WithContext(ctx)); reactErr != nil {
				slog.Warn("bwmarrin/discordgo Session.MessageReactionAdd",
					slog.String("err", reactErr.Error()))
			}
		}
		data := &discordgo.MessageSend{Content: text}
		if replyTo != "" {
			data.Reference = &discordgo.MessageReference{
				MessageID: replyTo,
				ChannelID: channelID,
			}
		}
		m, sendErr := b.api.ChannelMessageSendComplex(channelID, data, discordgo.WithContext(ctx))
		err = sendErr
		if m != nil {
			newID = m.ID
		}
	} else {
		_, editErr := b.api.ChannelMessageEdit(channelID, msgID, text, discordgo.WithContext(ctx))
		err = editErr
		newID = msgID
	}
	b.afterFlushStatus(channelID, status, newID, text, err)
	return err
}

func (b *Bot) afterFlushStatus(channelID string, status *ChannelStatus, newID, newText string, _ error) {
	b.statusMu.Lock()
	current, ok := b.statuses[channelID]
	if !ok || current != status {
		b.statusMu.Unlock()
		return
	}
	status.inflight = false
	status.lastFlush = time.Now()
	if status.messageID == "" && newID != "" {
		status.messageID = newID
	}
	status.lastText = newText

	if status.finishing {
		finishCtx := status.finishCtx
		msgID := status.messageID
		replyTo := status.replyTo
		reaction := status.reaction
		done := status.finishDone
		delete(b.statuses, channelID)
		b.statusMu.Unlock()

		_ = b.doFinishChat(finishCtx, channelID, msgID, replyTo, reaction)
		close(done)
		return
	}

	if status.hasPending && status.timer == nil {
		delay := statusMinInterval - time.Since(status.lastFlush)
		if delay <= 0 {
			delay = time.Millisecond
		}
		cid := channelID
		status.timer = time.AfterFunc(delay, func() {
			_ = b.flushStatus(cid)
		})
	}
	b.statusMu.Unlock()
}

func (b *Bot) doFinishChat(ctx context.Context, channelID, msgID, replyTo, reaction string) error {
	if replyTo != "" && reaction != "" {
		if reactErr := b.api.MessageReactionRemove(channelID, replyTo, reaction, "@me", discordgo.WithContext(ctx)); reactErr != nil {
			slog.Warn("bwmarrin/discordgo Session.MessageReactionRemove",
				slog.String("err", reactErr.Error()))
		}
	}
	if msgID == "" {
		return nil
	}
	return b.api.ChannelMessageDelete(channelID, msgID, discordgo.WithContext(ctx))
}
