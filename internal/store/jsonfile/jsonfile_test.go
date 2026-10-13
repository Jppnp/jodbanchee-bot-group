package jsonfile

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Jppnp/jodbanchee-bot-group/internal/ledger"
	"github.com/Jppnp/jodbanchee-bot-group/internal/store/storetest"
)

func TestContract(t *testing.T) {
	storetest.Run(t, func(t *testing.T) ledger.Store {
		s, err := Open(filepath.Join(t.TempDir(), "ledger.json"))
		if err != nil {
			t.Fatal(err)
		}
		return s
	})
}

func TestPersistsAcrossReopen(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "nested", "ledger.json")
	at := time.Date(2026, 10, 13, 1, 0, 0, 0, time.UTC)
	e := ledger.Entry{ChatID: "C1", UserID: "U1", Kind: ledger.Expense, Amount: 100, CreatedAt: at}

	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Add(ctx, []ledger.Entry{e, e, e}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := s.DeleteLatestByUser(ctx, "C1", "U1"); !ok || err != nil {
		t.Fatalf("delete: %v %v", ok, err)
	}

	s2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if got, _ := s2.List(ctx, "C1", at, at.Add(time.Second)); len(got) != 2 {
		t.Fatalf("after reopen got %d entries, want 2", len(got))
	}
	// IDs are never reused, even after a delete and a reopen.
	saved, err := s2.Add(ctx, []ledger.Entry{e})
	if err != nil || saved[0].ID != 4 {
		t.Errorf("next ID = %v (err %v), want 4", saved, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm&0o077 != 0 {
		t.Errorf("data file is readable by others: %v", perm)
	}
}

func TestFailedWriteLeavesNoTrace(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := Open(filepath.Join(dir, "ledger.json"))
	if err != nil {
		t.Fatal(err)
	}
	at := time.Now().UTC()
	e := ledger.Entry{ChatID: "C1", Kind: ledger.Income, Amount: 1, CreatedAt: at}

	// Make the directory read-only so the temp file cannot be created.
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o700) })
	if f, err := os.CreateTemp(dir, "probe"); err == nil {
		f.Close()
		os.Remove(f.Name())
		t.Skip("cannot make directory read-only (running as root?)")
	}

	if _, err := s.Add(ctx, []ledger.Entry{e, e}); err == nil {
		t.Fatal("expected write error")
	}
	if got, _ := s.List(ctx, "C1", at, at.Add(time.Second)); len(got) != 0 {
		t.Errorf("failed batch is visible in memory: %+v", got)
	}
}

func TestRejectsCorruptFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "l.json")
	os.WriteFile(path, []byte("{not json"), 0o600)
	if _, err := Open(path); err == nil {
		t.Error("expected error for corrupt file")
	}
}
