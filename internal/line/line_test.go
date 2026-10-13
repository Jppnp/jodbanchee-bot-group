package line

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestVerifySignature(t *testing.T) {
	body := []byte(`{"events":[]}`)
	// Known value: base64(HMAC-SHA256("secret", body)).
	const want = "pkK1lVPJPiJ+wPLziRD79xIxohl8AImYM8AEeM7IbzQ="
	if got := Sign("secret", body); got != want {
		t.Fatalf("Sign = %q, want %q", got, want)
	}
	if !VerifySignature("secret", body, want) {
		t.Error("valid signature rejected")
	}
	for name, tc := range map[string]struct {
		secret, sig string
		body        []byte
	}{
		"wrong secret": {"other", want, body},
		"changed body": {"secret", want, []byte(`{"events":[{}]}`)},
		"empty sig":    {"secret", "", body},
		"not base64":   {"secret", "%%%", body},
		"empty secret": {"", want, body},
	} {
		if VerifySignature(tc.secret, tc.body, tc.sig) {
			t.Errorf("%s: signature accepted", name)
		}
	}
}

func TestReply(t *testing.T) {
	var gotAuth, gotPath string
	var gotBody struct {
		ReplyToken string    `json:"replyToken"`
		Messages   []Message `json:"messages"`
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth, gotPath = r.Header.Get("Authorization"), r.URL.Path
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Write([]byte("{}"))
	}))
	defer srv.Close()

	c := NewClient("token123", srv.URL)
	msgs := []Message{Text("1"), Text("2"), Text("3"), Text("4"), Text("5"), Text("6")}
	if err := c.Reply(context.Background(), "rt", msgs); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer token123" || gotPath != "/v2/bot/message/reply" {
		t.Errorf("auth=%q path=%q", gotAuth, gotPath)
	}
	if gotBody.ReplyToken != "rt" || len(gotBody.Messages) != 5 {
		t.Errorf("body = %+v (LINE allows max 5 messages)", gotBody)
	}
}

func TestReplyError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"message":"Invalid reply token"}`))
	}))
	defer srv.Close()

	err := NewClient("t", srv.URL).Reply(context.Background(), "rt", []Message{Text("hi")})
	if err == nil || !strings.Contains(err.Error(), "Invalid reply token") {
		t.Errorf("err = %v", err)
	}
}

func TestTextTruncates(t *testing.T) {
	m := Text(strings.Repeat("ก", MaxTextRunes+10))
	if n := len([]rune(m.Text)); n != MaxTextRunes {
		t.Errorf("text has %d runes, want %d", n, MaxTextRunes)
	}
}

func TestSourceChatID(t *testing.T) {
	for _, tc := range []struct {
		src  Source
		want string
	}{
		{Source{Type: SourceGroup, GroupID: "C1", UserID: "U1"}, "C1"},
		{Source{Type: SourceRoom, RoomID: "R1", UserID: "U1"}, "R1"},
		{Source{Type: SourceUser, UserID: "U1"}, "U1"},
	} {
		if got := tc.src.ChatID(); got != tc.want {
			t.Errorf("%+v.ChatID() = %q, want %q", tc.src, got, tc.want)
		}
	}
}
