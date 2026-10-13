// Package ledger is the domain core: entries, periods and summaries.
// It knows nothing about LINE or how data is stored.
package ledger

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/Jppnp/jodbanchee-bot-group/internal/money"
)

// Kind says whether an entry is money in or money out.
type Kind string

const (
	Income  Kind = "income"
	Expense Kind = "expense"
)

// Entry is one recorded line in a chat's ledger.
type Entry struct {
	ID        int64        `json:"id"`
	ChatID    string       `json:"chat_id"` // LINE group, room or user ID
	UserID    string       `json:"user_id"` // who recorded it
	Kind      Kind         `json:"kind"`
	Amount    money.Satang `json:"amount"` // always positive
	Note      string       `json:"note"`
	CreatedAt time.Time    `json:"created_at"`
}

// Store persists entries. Implementations must be safe for concurrent use.
type Store interface {
	// Add saves all entries or none of them, assigns increasing IDs in order,
	// and returns the saved entries.
	Add(ctx context.Context, entries []Entry) ([]Entry, error)
	// List returns entries for chatID with from <= CreatedAt < to, oldest first.
	List(ctx context.Context, chatID string, from, to time.Time) ([]Entry, error)
	// DeleteLatestByUser removes the newest entry userID made in chatID.
	// ok is false when the user has no entries there.
	DeleteLatestByUser(ctx context.Context, chatID, userID string) (deleted Entry, ok bool, err error)
}

// NewEntry is what a caller wants to record.
type NewEntry struct {
	Kind   Kind
	Amount money.Satang
	Note   string
}

// MaxNoteRunes caps how long a note can be.
const MaxNoteRunes = 100

var ErrInvalidEntry = errors.New("ledger: invalid entry")

// Service holds the business rules on top of a Store.
type Service struct {
	store Store
	loc   *time.Location
	now   func() time.Time
}

// NewService creates a Service. loc decides where "today" and "this month"
// start and end (Asia/Bangkok for most users).
func NewService(store Store, loc *time.Location) *Service {
	return &Service{store: store, loc: loc, now: time.Now}
}

// SetClock replaces the clock; used by tests.
func (s *Service) SetClock(now func() time.Time) { s.now = now }

// Location returns the service's time zone.
func (s *Service) Location() *time.Location { return s.loc }

// Now returns the current time in the service's time zone.
func (s *Service) Now() time.Time { return s.now().In(s.loc) }

// Record saves new entries for one user in one chat.
func (s *Service) Record(ctx context.Context, chatID, userID string, items []NewEntry) ([]Entry, error) {
	if chatID == "" || len(items) == 0 {
		return nil, ErrInvalidEntry
	}
	// Validate everything first so a bad line never leaves a half-saved batch.
	for _, it := range items {
		if (it.Kind != Income && it.Kind != Expense) || it.Amount <= 0 || it.Amount > money.Max {
			return nil, ErrInvalidEntry
		}
	}
	// Truncate to microseconds: that is what Postgres stores, so every store
	// returns the same timestamps.
	now := s.now().UTC().Truncate(time.Microsecond)
	entries := make([]Entry, len(items))
	for i, it := range items {
		entries[i] = Entry{
			ChatID:    chatID,
			UserID:    userID,
			Kind:      it.Kind,
			Amount:    it.Amount,
			Note:      cleanNote(it.Note),
			CreatedAt: now,
		}
	}
	return s.store.Add(ctx, entries)
}

// Undo deletes the newest entry the user recorded in the chat.
func (s *Service) Undo(ctx context.Context, chatID, userID string) (Entry, bool, error) {
	return s.store.DeleteLatestByUser(ctx, chatID, userID)
}

// Summarize totals a chat's entries over a period.
func (s *Service) Summarize(ctx context.Context, chatID string, p Period) (Summary, error) {
	from, to := p.Range(s.Now())
	entries, err := s.store.List(ctx, chatID, from, to)
	if err != nil {
		return Summary{}, err
	}
	sum := Summary{Period: p, From: from, To: to, Count: len(entries)}
	byNote := map[string]money.Satang{}
	for _, e := range entries {
		switch e.Kind {
		case Income:
			sum.Income += e.Amount
		case Expense:
			sum.Expense += e.Amount
			byNote[e.Note] += e.Amount
		}
	}
	for note, amt := range byNote {
		sum.TopExpenses = append(sum.TopExpenses, NoteTotal{Note: note, Amount: amt})
	}
	sort.Slice(sum.TopExpenses, func(i, j int) bool {
		a, b := sum.TopExpenses[i], sum.TopExpenses[j]
		if a.Amount != b.Amount {
			return a.Amount > b.Amount
		}
		return a.Note < b.Note
	})
	if len(sum.TopExpenses) > 3 {
		sum.TopExpenses = sum.TopExpenses[:3]
	}
	return sum, nil
}

// Summary is the result of Summarize.
type Summary struct {
	Period      Period
	From, To    time.Time
	Income      money.Satang
	Expense     money.Satang
	Count       int
	TopExpenses []NoteTotal // biggest expense notes, at most 3
}

// Net is income minus expense.
func (s Summary) Net() money.Satang { return s.Income - s.Expense }

// NoteTotal is the total spent on one note, e.g. "ค่ากับข้าว".
type NoteTotal struct {
	Note   string
	Amount money.Satang
}

func cleanNote(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	r := []rune(s)
	if len(r) > MaxNoteRunes {
		s = string(r[:MaxNoteRunes])
	}
	return s
}
