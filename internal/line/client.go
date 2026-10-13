package line

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultAPIBase is the production Messaging API host.
const DefaultAPIBase = "https://api.line.me"

// MaxTextRunes is LINE's limit for one text message.
const MaxTextRunes = 5000

// Message is an outgoing message. Only text messages are used for now.
type Message struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// Text builds a text message, trimming it to LINE's length limit.
func Text(s string) Message {
	if r := []rune(s); len(r) > MaxTextRunes {
		s = string(r[:MaxTextRunes-1]) + "…"
	}
	return Message{Type: "text", Text: s}
}

// Client calls the Messaging API.
type Client struct {
	accessToken string
	apiBase     string
	http        *http.Client
}

// NewClient creates a client. apiBase is usually DefaultAPIBase.
func NewClient(channelAccessToken, apiBase string) *Client {
	if apiBase == "" {
		apiBase = DefaultAPIBase
	}
	return &Client{
		accessToken: channelAccessToken,
		apiBase:     apiBase,
		http:        &http.Client{Timeout: 10 * time.Second},
	}
}

// Reply answers an event using its reply token. Replies do not count toward
// the monthly message quota, which is why this bot only ever replies.
// LINE accepts at most 5 messages per reply.
func (c *Client) Reply(ctx context.Context, replyToken string, msgs []Message) error {
	if replyToken == "" || len(msgs) == 0 {
		return nil
	}
	if len(msgs) > 5 {
		msgs = msgs[:5]
	}
	body, err := json.Marshal(struct {
		ReplyToken string    `json:"replyToken"`
		Messages   []Message `json:"messages"`
	}{replyToken, msgs})
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.apiBase+"/v2/bot/message/reply", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.accessToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("line: reply: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf("line: reply: status %d: %s", resp.StatusCode, bytes.TrimSpace(detail))
	}
	return nil
}
