package bot

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Jppnp/jodbanchee-bot-group/internal/ledger"
	"github.com/Jppnp/jodbanchee-bot-group/internal/line"
	"github.com/Jppnp/jodbanchee-bot-group/internal/store/jsonfile"
)

const secret = "test-secret"

type fakeReplier struct {
	mu      sync.Mutex
	replies map[string][]line.Message // by reply token
}

func (f *fakeReplier) Reply(_ context.Context, token string, msgs []line.Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.replies == nil {
		f.replies = map[string][]line.Message{}
	}
	f.replies[token] = append(f.replies[token], msgs...)
	return nil
}

func (f *fakeReplier) text(token string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var parts []string
	for _, m := range f.replies[token] {
		parts = append(parts, m.Text)
	}
	return strings.Join(parts, "\n---\n")
}

type harness struct {
	h       *WebhookHandler
	replier *fakeReplier
	svc     *ledger.Service
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	store, err := jsonfile.Open(filepath.Join(t.TempDir(), "ledger.json"))
	if err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("Asia/Bangkok")
	svc := ledger.NewService(store, loc)
	svc.SetClock(func() time.Time { return time.Date(2026, 10, 13, 18, 40, 0, 0, loc) })
	r := &fakeReplier{}
	return &harness{h: NewWebhookHandler(secret, New(svc, nil), r, nil), replier: r, svc: svc}
}

func (hs *harness) post(t *testing.T, events ...line.Event) *httptest.ResponseRecorder {
	t.Helper()
	body, _ := json.Marshal(line.WebhookRequest{Destination: "Ubot", Events: events})
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(string(body)))
	req.Header.Set(line.SignatureHeader, line.Sign(secret, body))
	rec := httptest.NewRecorder()
	hs.h.ServeHTTP(rec, req)
	return rec
}

func textEvent(id, token, user, text string) line.Event {
	return line.Event{
		Type:           line.EventMessage,
		Mode:           "active",
		WebhookEventID: id,
		ReplyToken:     token,
		Source:         line.Source{Type: line.SourceGroup, GroupID: "Cfamily", UserID: user},
		Message:        &line.EventMessageBody{ID: id, Type: "text", Text: text},
	}
}

func TestRejectsBadSignature(t *testing.T) {
	hs := newHarness(t)
	req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(`{"events":[]}`))
	req.Header.Set(line.SignatureHeader, line.Sign("wrong", []byte(`{"events":[]}`)))
	rec := httptest.NewRecorder()
	hs.h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", rec.Code)
	}
}

func TestVerifyRequestFromConsole(t *testing.T) {
	// The "Verify" button in LINE Developers Console sends no events.
	hs := newHarness(t)
	if rec := hs.post(t); rec.Code != http.StatusOK {
		t.Errorf("status = %d, want 200", rec.Code)
	}
}

func TestJoinSendsWelcome(t *testing.T) {
	hs := newHarness(t)
	hs.post(t, line.Event{
		Type: line.EventJoin, Mode: "active", WebhookEventID: "e1", ReplyToken: "rt-join",
		Source: line.Source{Type: line.SourceGroup, GroupID: "Cfamily"},
	})
	got := hs.replier.text("rt-join")
	if !strings.Contains(got, "สวัสดี") || !strings.Contains(got, "วิธีใช้") {
		t.Errorf("join reply = %q", got)
	}
}

func TestConversationFlow(t *testing.T) {
	hs := newHarness(t)

	hs.post(t,
		textEvent("e1", "rt1", "Umom", "สวัสดีจ้าลูกๆ"),
		textEvent("e2", "rt2", "Umom", "+500 ขายผัก"),
		textEvent("e3", "rt3", "Ukid", "-120 ค่ากับข้าว\n-30 ค่าน้ำแข็ง"),
		textEvent("e4", "rt4", "Umom", "สรุป"),
	)

	if got := hs.replier.text("rt1"); got != "" {
		t.Errorf("bot replied to normal chat: %q", got)
	}
	if got := hs.replier.text("rt2"); !strings.Contains(got, "รายรับ 500 บาท · ขายผัก") {
		t.Errorf("record reply = %q", got)
	}
	if got := hs.replier.text("rt3"); !strings.Contains(got, "บันทึก 2 รายการ") || !strings.Contains(got, "คงเหลือ +350") {
		t.Errorf("batch reply = %q", got)
	}
	want := "📊 สรุปเดือนตุลาคม 2569\nรายรับ 500 บาท\nรายจ่าย 150 บาท\nคงเหลือ +350 บาท\n(3 รายการ)\n\nจ่ายมากสุด\n1. ค่ากับข้าว 120 บาท\n2. ค่าน้ำแข็ง 30 บาท"
	if got := hs.replier.text("rt4"); got != want {
		t.Errorf("summary reply =\n%s\nwant\n%s", got, want)
	}

	hs.post(t, textEvent("e5", "rt5", "Umom", "ลบล่าสุด"))
	if got := hs.replier.text("rt5"); !strings.Contains(got, "ลบแล้ว: รายรับ 500 บาท · ขายผัก") || !strings.Contains(got, "18:40") {
		t.Errorf("undo reply = %q", got)
	}
}

func TestSkipsRedeliveredAndStandbyEvents(t *testing.T) {
	hs := newHarness(t)
	ev := textEvent("same-id", "rt1", "Umom", "+100 a")
	hs.post(t, ev)
	ev.ReplyToken = "rt2"
	ev.DeliveryContext.IsRedelivery = true
	hs.post(t, ev)

	standby := textEvent("e9", "rt3", "Umom", "+100 b")
	standby.Mode = "standby"
	hs.post(t, standby)

	sum, _ := hs.svc.Summarize(context.Background(), "Cfamily", ledger.ThisMonth)
	if sum.Count != 1 {
		t.Errorf("Count = %d, want 1 (duplicate or standby event was recorded)", sum.Count)
	}
	if hs.replier.text("rt2") != "" || hs.replier.text("rt3") != "" {
		t.Error("replied to a skipped event")
	}
}

func TestIgnoresNonTextMessages(t *testing.T) {
	hs := newHarness(t)
	ev := textEvent("e1", "rt1", "Umom", "")
	ev.Message.Type = "sticker"
	hs.post(t, ev)
	if got := hs.replier.text("rt1"); got != "" {
		t.Errorf("replied to sticker: %q", got)
	}
}

func TestRecentIDsEvictsOldest(t *testing.T) {
	r := newRecentIDs(2)
	r.add("a")
	r.add("b")
	r.add("c") // evicts a
	if !r.add("a") {
		t.Error("a should have been evicted")
	}
	if r.add("c") {
		t.Error("c should still be remembered")
	}
}
