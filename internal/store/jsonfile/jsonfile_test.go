package jsonfile

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/Jppnp/jodbanchee-bot-group/internal/ledger"
)

func TestPersistsAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "nested", "ledger.json")

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 10, 13, 1, 0, 0, 0, time.UTC)
	for i := range 3 {
		e, err := s.Add(ctx, ledger.Entry{ChatID: "C1", UserID: "U1", Kind: ledger.Expense, Amount: 100, CreatedAt: at})
		if err != nil {
			t.Fatal(err)
		}
		if e.ID != int64(i+1) {
			t.Errorf("ID = %d, want %d", e.ID, i+1)
		}
	}
	if _, ok, err := s.DeleteLatestByUser(ctx, "C1", "U1"); !ok || err != nil {
		t.Fatalf("delete: %v %v", ok, err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := s2.List(ctx, "C1", at, at.Add(time.Second))
	if len(got) != 2 {
		t.Fatalf("after reopen got %d entries, want 2", len(got))
	}
	// IDs keep increasing after a delete and a reopen.
	e, _ := s2.Add(ctx, ledger.Entry{ChatID: "C1", CreatedAt: at})
	if e.ID != 4 {
		t.Errorf("next ID = %d, want 4", e.ID)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("data file is readable by others: %v", perm)
	}
}

func TestListRangeIsHalfOpen(t *testing.T) {
	ctx := context.Background()
	s, _ := Open(filepath.Join(t.TempDir(), "l.json"))
	from := time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)
	to := from.AddDate(0, 1, 0)
	for _, at := range []time.Time{from.Add(-time.Nanosecond), from, to.Add(-time.Nanosecond), to} {
		s.Add(ctx, ledger.Entry{ChatID: "C1", CreatedAt: at})
	}
	if got, _ := s.List(ctx, "C1", from, to); len(got) != 2 {
		t.Errorf("List returned %d entries, want 2", len(got))
	}
}

func TestRejectsCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "l.json")
	os.WriteFile(path, []byte("{not json"), 0o600)
	if _, err := Open(path); err == nil {
		t.Error("expected error for corrupt file")
	}
}

func TestConcurrentAdds(t *testing.T) {
	ctx := context.Background()
	s, _ := Open(filepath.Join(t.TempDir(), "l.json"))
	at := time.Now()

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Add(ctx, ledger.Entry{ChatID: "C1", CreatedAt: at}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	if got, _ := s.List(ctx, "C1", at, at.Add(time.Second)); len(got) != 20 {
		t.Errorf("got %d entries, want 20", len(got))
	}
}
