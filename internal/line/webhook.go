// Package line is a small, dependency-free client for the parts of the LINE
// Messaging API this bot uses: webhook signature checks, webhook events and
// the reply endpoint.
//
// API reference: https://developers.line.biz/en/reference/messaging-api/
package line

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
)

// SignatureHeader is the header LINE signs every webhook request with.
const SignatureHeader = "X-Line-Signature"

// VerifySignature reports whether signature is the base64 HMAC-SHA256 of body
// keyed with the channel secret.
func VerifySignature(channelSecret string, body []byte, signature string) bool {
	if channelSecret == "" || signature == "" {
		return false
	}
	got, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, []byte(channelSecret))
	mac.Write(body)
	return hmac.Equal(got, mac.Sum(nil))
}

// Sign returns the signature LINE would send for body. Useful for tests and
// local tools that simulate LINE.
func Sign(channelSecret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(channelSecret))
	mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// WebhookRequest is the body LINE posts to the webhook URL.
type WebhookRequest struct {
	Destination string  `json:"destination"`
	Events      []Event `json:"events"`
}

// Event types the bot cares about.
const (
	EventMessage = "message"
	EventJoin    = "join"   // bot was added to a group or room
	EventFollow  = "follow" // someone added the bot as a friend
)

// Event is one webhook event. Fields unused by this bot are omitted.
type Event struct {
	Type            string            `json:"type"`
	Mode            string            `json:"mode"` // "active" or "standby"
	Timestamp       int64             `json:"timestamp"`
	Source          Source            `json:"source"`
	WebhookEventID  string            `json:"webhookEventId"`
	DeliveryContext DeliveryContext   `json:"deliveryContext"`
	ReplyToken      string            `json:"replyToken,omitempty"`
	Message         *EventMessageBody `json:"message,omitempty"`
}

// DeliveryContext tells whether LINE is re-sending an event.
type DeliveryContext struct {
	IsRedelivery bool `json:"isRedelivery"`
}

// Source types.
const (
	SourceUser  = "user"
	SourceGroup = "group"
	SourceRoom  = "room"
)

// Source says where an event came from.
type Source struct {
	Type    string `json:"type"`
	UserID  string `json:"userId,omitempty"`
	GroupID string `json:"groupId,omitempty"`
	RoomID  string `json:"roomId,omitempty"`
}

// ChatID identifies the conversation: the group, the room, or the 1:1 user.
// Each chat keeps its own ledger.
func (s Source) ChatID() string {
	switch s.Type {
	case SourceGroup:
		return s.GroupID
	case SourceRoom:
		return s.RoomID
	default:
		return s.UserID
	}
}

// EventMessageBody is the message inside a message event.
type EventMessageBody struct {
	ID   string `json:"id"`
	Type string `json:"type"` // "text", "image", "sticker", ...
	Text string `json:"text,omitempty"`
}
