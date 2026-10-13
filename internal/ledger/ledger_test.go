package ledger_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/jodbanchee/jodbanchee/internal/ledger"
	"github.com/jodbanchee/jodbanchee/internal/money"
	"github.com/jodbanchee/jodbanchee/internal/store/jsonfile"
)

var bangkok = mustLoad("Asia/Bangkok")

func mustLoad(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err)
	}
	return loc
}

func TestPeriodRange(t *testing.T) {
	now := time.Date(2026, time.March, 1, 0, 30, 0, 0, bangkok)
	tests := []struct {
		p        ledger.Period
		from, to time.Time
	}{
		{ledger.Today, date(2026, 3, 1), date(2026, 3, 2)},
		{ledger.Yesterday, date(2026, 2, 28), date(2026, 3, 1)},
		{ledger.ThisMonth, date(2026, 3, 1), date(2026, 4, 1)},
		{ledger.LastMonth, date(2026, 2, 1), date(2026, 3, 1)},
	}
	for _, tt := range tests {
		from, to := tt.p.Range(now)
		if !from.Equal(tt.from) || !to.Equal(tt.to) {
			t.Errorf("%s.Range = [%v, %v), want [%v, %v)", tt.p, from, to, tt.from, tt.to)
		}
	}
}

func date(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, bangkok) }

func newService(t *testing.T) (*ledger.Service, *time.Time) {
	t.Helper()
	store, err := jsonfile.Open(filepath.Join(t.TempDir(), "ledger.json"))
	if err != nil {
		t.Fatal(err)
	}
	svc := ledger.NewService(store, bangkok)
	now := time.Date(2026, time.October, 13, 9, 0, 0, 0, bangkok)
	svc.SetClock(func() time.Time { return now })
	return svc, &now
}

func TestRecordAndSummarize(t *testing.T) {
	ctx := context.Background()
	svc, now := newService(t)

	record := func(chat string, items ...ledger.NewEntry) {
		t.Helper()
		if _, err := svc.Record(ctx, chat, "Umom", items); err != nil {
			t.Fatal(err)
		}
	}

	// Last month, at 23:30 Bangkok time on 30 Sep (16:30 UTC).
	*now = time.Date(2026, time.September, 30, 23, 30, 0, 0, bangkok)
	record("C1", ledger.NewEntry{Kind: ledger.Expense, Amount: 99900, Note: "ค่าเช่า"})

	*now = time.Date(2026, time.October, 12, 18, 0, 0, 0, bangkok)
	record("C1",
		ledger.NewEntry{Kind: ledger.Income, Amount: 50000, Note: "ขายผัก"},
		ledger.NewEntry{Kind: ledger.Expense, Amount: 12000, Note: "ค่ากับข้าว"},
	)
	*now = time.Date(2026, time.October, 13, 8, 0, 0, 0, bangkok)
	record("C1",
		ledger.NewEntry{Kind: ledger.Expense, Amount: 8000, Note: "ค่ากับข้าว"},
		ledger.NewEntry{Kind: ledger.Expense, Amount: 15000, Note: "ค่าไฟ"},
		ledger.NewEntry{Kind: ledger.Expense, Amount: 1000, Note: ""},
		ledger.NewEntry{Kind: ledger.Expense, Amount: 500, Note: "ขนม"},
	)
	record("C2", ledger.NewEntry{Kind: ledger.Income, Amount: 1_000_000, Note: "other chat"})

	month, err := svc.Summarize(ctx, "C1", ledger.ThisMonth)
	if err != nil {
		t.Fatal(err)
	}
	if month.Count != 6 || month.Income != 50000 || month.Expense != 36500 || month.Net() != 13500 {
		t.Errorf("this month = %+v", month)
	}
	wantTop := []ledger.NoteTotal{{"ค่ากับข้าว", 20000}, {"ค่าไฟ", 15000}, {"", 1000}}
	if len(month.TopExpenses) != 3 {
		t.Fatalf("TopExpenses = %+v", month.TopExpenses)
	}
	for i, w := range wantTop {
		if month.TopExpenses[i] != w {
			t.Errorf("TopExpenses[%d] = %+v, want %+v", i, month.TopExpenses[i], w)
		}
	}

	today, _ := svc.Summarize(ctx, "C1", ledger.Today)
	if today.Count != 4 || today.Expense != 24500 {
		t.Errorf("today = %+v", today)
	}
	yesterday, _ := svc.Summarize(ctx, "C1", ledger.Yesterday)
	if yesterday.Count != 2 || yesterday.Net() != 38000 {
		t.Errorf("yesterday = %+v", yesterday)
	}
	last, _ := svc.Summarize(ctx, "C1", ledger.LastMonth)
	if last.Count != 1 || last.Expense != 99900 {
		t.Errorf("last month = %+v (Bangkok month boundary not respected?)", last)
	}
}

func TestRecordValidatesWholeBatch(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)

	_, err := svc.Record(ctx, "C1", "U1", []ledger.NewEntry{
		{Kind: ledger.Income, Amount: 100},
		{Kind: ledger.Expense, Amount: 0}, // invalid
	})
	if err == nil {
		t.Fatal("expected error for zero amount")
	}
	sum, _ := svc.Summarize(ctx, "C1", ledger.ThisMonth)
	if sum.Count != 0 {
		t.Errorf("batch was partly saved: %+v", sum)
	}

	if _, err := svc.Record(ctx, "C1", "U1", []ledger.NewEntry{{Kind: ledger.Income, Amount: money.Max + 1}}); err == nil {
		t.Error("expected error for amount over max")
	}
	if _, err := svc.Record(ctx, "", "U1", []ledger.NewEntry{{Kind: ledger.Income, Amount: 1}}); err == nil {
		t.Error("expected error for empty chat ID")
	}
}

func TestRecordCleansNote(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)
	long := ""
	for range 150 {
		long += "ก"
	}
	saved, err := svc.Record(ctx, "C1", "U1", []ledger.NewEntry{
		{Kind: ledger.Expense, Amount: 100, Note: "  ค่า   กับข้าว  "},
		{Kind: ledger.Expense, Amount: 100, Note: long},
	})
	if err != nil {
		t.Fatal(err)
	}
	if saved[0].Note != "ค่า กับข้าว" {
		t.Errorf("note = %q", saved[0].Note)
	}
	if n := len([]rune(saved[1].Note)); n != ledger.MaxNoteRunes {
		t.Errorf("long note has %d runes, want %d", n, ledger.MaxNoteRunes)
	}
}

func TestUndoOnlyTouchesOwnEntries(t *testing.T) {
	ctx := context.Background()
	svc, _ := newService(t)

	svc.Record(ctx, "C1", "Umom", []ledger.NewEntry{{Kind: ledger.Expense, Amount: 100, Note: "a"}})
	svc.Record(ctx, "C1", "Udad", []ledger.NewEntry{{Kind: ledger.Expense, Amount: 200, Note: "b"}})
	svc.Record(ctx, "C2", "Umom", []ledger.NewEntry{{Kind: ledger.Expense, Amount: 300, Note: "c"}})

	e, ok, err := svc.Undo(ctx, "C1", "Umom")
	if err != nil || !ok || e.Note != "a" {
		t.Fatalf("Undo = %+v, %v, %v; want entry a", e, ok, err)
	}
	if _, ok, _ := svc.Undo(ctx, "C1", "Umom"); ok {
		t.Error("second undo should find nothing")
	}
	sum, _ := svc.Summarize(ctx, "C1", ledger.ThisMonth)
	if sum.Count != 1 || sum.Expense != 200 {
		t.Errorf("dad's entry should remain: %+v", sum)
	}
}
