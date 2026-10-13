package bot

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/jodbanchee/jodbanchee/internal/line"
)

// Replier sends reply messages. *line.Client implements it.
type Replier interface {
	Reply(ctx context.Context, replyToken string, msgs []line.Message) error
}

// maxBodyBytes caps webhook request bodies; real ones are a few KB.
const maxBodyBytes = 1 << 20

// WebhookHandler is the HTTP endpoint LINE calls.
type WebhookHandler struct {
	secret  string
	bot     *Bot
	replier Replier
	log     *slog.Logger
	seen    *recentIDs
}

// NewWebhookHandler wires the bot to LINE.
func NewWebhookHandler(channelSecret string, b *Bot, r Replier, log *slog.Logger) *WebhookHandler {
	if log == nil {
		log = slog.Default()
	}
	return &WebhookHandler{secret: channelSecret, bot: b, replier: r, log: log, seen: newRecentIDs(1024)}
}

func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	if !line.VerifySignature(h.secret, body, r.Header.Get(line.SignatureHeader)) {
		h.log.Warn("rejected webhook with bad signature", "remote", r.RemoteAddr)
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	var req line.WebhookRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	// Keep working even if LINE closes the connection early: a saved entry
	// should still get its confirmation reply.
	ctx, cancel := context.WithTimeout(context.WithoutCancel(r.Context()), 20*time.Second)
	defer cancel()

	for _, ev := range req.Events {
		if ev.Mode == "standby" {
			continue // another channel is handling this chat
		}
		if ev.WebhookEventID != "" && !h.seen.add(ev.WebhookEventID) {
			continue // redelivered event we already handled
		}
		msgs := h.bot.Respond(ctx, ev)
		if len(msgs) == 0 {
			continue
		}
		if err := h.replier.Reply(ctx, ev.ReplyToken, msgs); err != nil {
			h.log.ErrorContext(ctx, "send reply", "err", err, "event_type", ev.Type)
		}
	}
	w.WriteHeader(http.StatusOK)
}

// recentIDs remembers the last n webhook event IDs.
type recentIDs struct {
	mu   sync.Mutex
	ids  map[string]struct{}
	ring []string
	next int
}

func newRecentIDs(n int) *recentIDs {
	return &recentIDs{ids: make(map[string]struct{}, n), ring: make([]string, n)}
}

// add records id and reports whether it was new.
func (r *recentIDs) add(id string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.ids[id]; ok {
		return false
	}
	if old := r.ring[r.next]; old != "" {
		delete(r.ids, old)
	}
	r.ring[r.next] = id
	r.ids[id] = struct{}{}
	r.next = (r.next + 1) % len(r.ring)
	return true
}
