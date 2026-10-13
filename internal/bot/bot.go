// Package bot decides how to answer LINE events. It turns parsed commands
// into ledger calls and formats the Thai replies.
package bot

import (
	"context"
	"log/slog"

	"github.com/jodbanchee/jodbanchee/internal/ledger"
	"github.com/jodbanchee/jodbanchee/internal/line"
	"github.com/jodbanchee/jodbanchee/internal/parser"
)

// Bot answers events. It does not talk to LINE itself, which keeps it easy
// to test and lets the dev chat tool reuse it.
type Bot struct {
	ledger *ledger.Service
	log    *slog.Logger
}

// New creates a Bot.
func New(svc *ledger.Service, log *slog.Logger) *Bot {
	if log == nil {
		log = slog.Default()
	}
	return &Bot{ledger: svc, log: log}
}

// Respond returns the messages to reply with, or nil to stay quiet.
func (b *Bot) Respond(ctx context.Context, ev line.Event) []line.Message {
	switch ev.Type {
	case line.EventJoin, line.EventFollow:
		return []line.Message{line.Text(welcomeText), line.Text(helpText)}
	case line.EventMessage:
		if ev.Message == nil || ev.Message.Type != "text" {
			return nil
		}
		return b.handleText(ctx, ev.Source, ev.Message.Text)
	default:
		return nil
	}
}

func (b *Bot) handleText(ctx context.Context, src line.Source, text string) []line.Message {
	chatID, userID := src.ChatID(), src.UserID
	if chatID == "" {
		return nil
	}

	cmd := parser.Parse(text)
	switch cmd.Action {
	case parser.Help:
		return reply(helpText)

	case parser.Record:
		saved, err := b.ledger.Record(ctx, chatID, userID, cmd.Entries)
		if err != nil {
			b.log.ErrorContext(ctx, "record entries", "err", err, "chat_type", src.Type)
			return reply(errSaveText)
		}
		month, err := b.ledger.Summarize(ctx, chatID, ledger.ThisMonth)
		if err != nil {
			b.log.ErrorContext(ctx, "summarize after record", "err", err)
			return reply(errReadText)
		}
		return reply(recordedText(saved, month))

	case parser.Summary:
		sum, err := b.ledger.Summarize(ctx, chatID, cmd.Period)
		if err != nil {
			b.log.ErrorContext(ctx, "summarize", "err", err)
			return reply(errReadText)
		}
		return reply(summaryText(sum))

	case parser.Undo:
		if userID == "" {
			return reply(noUserText)
		}
		e, ok, err := b.ledger.Undo(ctx, chatID, userID)
		switch {
		case err != nil:
			b.log.ErrorContext(ctx, "undo", "err", err)
			return reply(errSaveText)
		case !ok:
			return reply(nothingToUndo)
		}
		return reply(undoneText(e, b.ledger.Location()))

	default:
		return nil // ordinary conversation
	}
}

func reply(text string) []line.Message { return []line.Message{line.Text(text)} }
