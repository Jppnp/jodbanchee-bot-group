// Package jsonfile is a ledger.Store kept in a single JSON file.
//
// A family ledger holds a few thousand entries a year, so loading everything
// into memory and rewriting the file on each change is simple and fast enough.
// Writes go to a temp file that is then renamed, so a crash never leaves a
// half-written file behind.
package jsonfile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/Jppnp/jodbanchee-bot-group/internal/ledger"
)

const formatVersion = 1

type fileData struct {
	Version int            `json:"version"`
	LastID  int64          `json:"last_id"`
	Entries []ledger.Entry `json:"entries"`
}

// Store implements ledger.Store.
type Store struct {
	path string

	mu   sync.RWMutex
	data fileData
}

var _ ledger.Store = (*Store)(nil)

// Open loads the file at path, creating it (and its directory) if missing.
func Open(path string) (*Store, error) {
	s := &Store{path: path, data: fileData{Version: formatVersion}}

	raw, err := os.ReadFile(path)
	switch {
	case errors.Is(err, os.ErrNotExist):
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, fmt.Errorf("jsonfile: create data dir: %w", err)
		}
		if err := s.flush(); err != nil {
			return nil, err
		}
		return s, nil
	case err != nil:
		return nil, fmt.Errorf("jsonfile: read %s: %w", path, err)
	}

	if err := json.Unmarshal(raw, &s.data); err != nil {
		return nil, fmt.Errorf("jsonfile: parse %s: %w", path, err)
	}
	if s.data.Version != formatVersion {
		return nil, fmt.Errorf("jsonfile: %s has unsupported version %d", path, s.data.Version)
	}
	return s, nil
}

// Add implements ledger.Store.
func (s *Store) Add(_ context.Context, entries []ledger.Entry) ([]ledger.Entry, error) {
	if len(entries) == 0 {
		return nil, nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	prevLen, prevID := len(s.data.Entries), s.data.LastID
	saved := make([]ledger.Entry, len(entries))
	for i, e := range entries {
		s.data.LastID++
		e.ID = s.data.LastID
		saved[i] = e
	}
	s.data.Entries = append(s.data.Entries, saved...)

	// One write for the whole batch makes it all-or-nothing.
	if err := s.flush(); err != nil {
		s.data.Entries = s.data.Entries[:prevLen]
		s.data.LastID = prevID
		return nil, err
	}
	return saved, nil
}

// List implements ledger.Store.
func (s *Store) List(_ context.Context, chatID string, from, to time.Time) ([]ledger.Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var out []ledger.Entry
	for _, e := range s.data.Entries {
		if e.ChatID == chatID && !e.CreatedAt.Before(from) && e.CreatedAt.Before(to) {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

// DeleteLatestByUser implements ledger.Store.
func (s *Store) DeleteLatestByUser(_ context.Context, chatID, userID string) (ledger.Entry, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := len(s.data.Entries) - 1; i >= 0; i-- {
		e := s.data.Entries[i]
		if e.ChatID != chatID || e.UserID != userID {
			continue
		}
		before := s.data.Entries
		s.data.Entries = append(append([]ledger.Entry(nil), before[:i]...), before[i+1:]...)
		if err := s.flush(); err != nil {
			s.data.Entries = before
			return ledger.Entry{}, false, err
		}
		return e, true, nil
	}
	return ledger.Entry{}, false, nil
}

// flush writes the whole store to disk atomically. Callers hold s.mu.
func (s *Store) flush() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return fmt.Errorf("jsonfile: encode: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".ledger-*.tmp")
	if err != nil {
		return fmt.Errorf("jsonfile: create temp file: %w", err)
	}
	defer os.Remove(tmp.Name()) // no-op after a successful rename

	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		return fmt.Errorf("jsonfile: write: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return fmt.Errorf("jsonfile: sync: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("jsonfile: close: %w", err)
	}
	if err := os.Rename(tmp.Name(), s.path); err != nil {
		return fmt.Errorf("jsonfile: replace %s: %w", s.path, err)
	}
	return nil
}
