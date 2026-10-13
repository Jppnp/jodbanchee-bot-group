// Package storetest is a contract test suite every ledger.Store must pass.
// Each store package calls Run from its own tests.
package storetest

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/Jppnp/jodbanchee-bot-group/internal/ledger"
	"github.com/Jppnp/jodbanchee-bot-group/internal/money"
)

// NewStore returns an empty store for one test.
type NewStore func(t *testing.T) ledger.Store

// Run runs the full contract against stores made by newStore.
func Run(t *testing.T, newStore NewStore) {
	tests := map[string]func(*testing.T, ledger.Store){
		"AddAssignsIncreasingIDs":     testAddAssignsIDs,
		"AddEmptyBatch":               testAddEmpty,
		"ListFiltersChatAndRange":     testListFilters,
		"ListOrdersOldestFirst":       testListOrder,
		"RoundTripsAllFields":         testRoundTrip,
		"DeleteLatestByUser":          testDeleteLatest,
		"DeleteLatestByUserWhenEmpty": testDeleteEmpty,
		"ConcurrentAdds":              testConcurrentAdds,
	}
	for name, fn := range tests {
		t.Run(name, func(t *testing.T) { fn(t, newStore(t)) })
	}
}

var base = time.Date(2026, 10, 13, 5, 0, 0, 0, time.UTC)

// entry makes an expense of baht*100 baht (amounts double as labels in tests).
func entry(chat, user string, baht int64, at time.Time) ledger.Entry {
	return ledger.Entry{ChatID: chat, UserID: user, Kind: ledger.Expense, Amount: money.Satang(baht * 100 * 100), Note: "n", CreatedAt: at}
}

func add(t *testing.T, s ledger.Store, entries ...ledger.Entry) []ledger.Entry {
	t.Helper()
	saved, err := s.Add(context.Background(), entries)
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if len(saved) != len(entries) {
		t.Fatalf("Add returned %d entries, want %d", len(saved), len(entries))
	}
	return saved
}

func list(t *testing.T, s ledger.Store, chat string, from, to time.Time) []ledger.Entry {
	t.Helper()
	got, err := s.List(context.Background(), chat, from, to)
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	return got
}

func testAddAssignsIDs(t *testing.T, s ledger.Store) {
	a := add(t, s, entry("C1", "U1", 1, base), entry("C1", "U1", 2, base), entry("C1", "U1", 3, base))
	b := add(t, s, entry("C1", "U1", 4, base))
	all := append(a, b...)
	for i := 1; i < len(all); i++ {
		if all[i].ID <= all[i-1].ID {
			t.Fatalf("IDs not increasing: %d then %d", all[i-1].ID, all[i].ID)
		}
	}
	for i, e := range a {
		if want := money.Satang((i + 1) * 100 * 100); e.Amount != want {
			t.Errorf("entry %d amount = %d, want %d (order not kept)", i, e.Amount, want)
		}
	}
}

func testAddEmpty(t *testing.T, s ledger.Store) {
	saved, err := s.Add(context.Background(), nil)
	if err != nil || len(saved) != 0 {
		t.Errorf("Add(nil) = %v, %v; want empty, nil", saved, err)
	}
}

func testListFilters(t *testing.T, s ledger.Store) {
	from, to := base, base.Add(24*time.Hour)
	add(t, s,
		entry("C1", "U1", 1, from.Add(-time.Microsecond)), // before range
		entry("C1", "U1", 2, from),                        // first instant: included
		entry("C1", "U1", 3, to.Add(-time.Microsecond)),   // last instant: included
		entry("C1", "U1", 4, to),                          // end is exclusive
		entry("C2", "U1", 5, from.Add(time.Hour)),         // other chat
	)
	got := list(t, s, "C1", from, to)
	if len(got) != 2 || got[0].Amount != 200*100 || got[1].Amount != 300*100 {
		t.Errorf("List = %+v, want amounts 200 and 300 baht", got)
	}
	if got := list(t, s, "nobody", from, to); len(got) != 0 {
		t.Errorf("List for unknown chat = %+v, want empty", got)
	}
}

func testListOrder(t *testing.T, s ledger.Store) {
	add(t, s, entry("C1", "U1", 2, base.Add(2*time.Minute)))
	add(t, s, entry("C1", "U1", 1, base.Add(1*time.Minute)))
	got := list(t, s, "C1", base, base.Add(time.Hour))
	if len(got) != 2 || !got[0].CreatedAt.Before(got[1].CreatedAt) {
		t.Errorf("List not ordered by CreatedAt: %+v", got)
	}
}

func testRoundTrip(t *testing.T, s ledger.Store) {
	in := ledger.Entry{
		ChatID: "Cgroup", UserID: "Umom", Kind: ledger.Income, Amount: 125050,
		Note: "ขายผัก 🥬", CreatedAt: base.Add(123456 * time.Microsecond),
	}
	saved := add(t, s, in)[0]
	got := list(t, s, "Cgroup", base, base.Add(time.Second))
	if len(got) != 1 {
		t.Fatalf("List = %+v", got)
	}
	if !got[0].CreatedAt.Equal(in.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got[0].CreatedAt, in.CreatedAt)
	}
	got[0].CreatedAt, saved.CreatedAt = in.CreatedAt, in.CreatedAt // compare the rest exactly
	in.ID = saved.ID
	if !reflect.DeepEqual(got[0], in) || !reflect.DeepEqual(saved, in) {
		t.Errorf("round trip:\n got   %+v\n saved %+v\n want  %+v", got[0], saved, in)
	}
}

func testDeleteLatest(t *testing.T, s ledger.Store) {
	ctx := context.Background()
	add(t, s, entry("C1", "Umom", 1, base), entry("C1", "Umom", 2, base))
	add(t, s, entry("C1", "Udad", 3, base))
	add(t, s, entry("C2", "Umom", 4, base))

	e, ok, err := s.DeleteLatestByUser(ctx, "C1", "Umom")
	if err != nil || !ok || e.Amount != 200*100 {
		t.Fatalf("DeleteLatestByUser = %+v, %v, %v; want the 200 baht entry", e, ok, err)
	}
	got := list(t, s, "C1", base, base.Add(time.Second))
	if len(got) != 2 {
		t.Errorf("after delete C1 has %d entries, want 2: %+v", len(got), got)
	}
	if got := list(t, s, "C2", base, base.Add(time.Second)); len(got) != 1 {
		t.Errorf("delete touched another chat: %+v", got)
	}
}

func testDeleteEmpty(t *testing.T, s ledger.Store) {
	_, ok, err := s.DeleteLatestByUser(context.Background(), "C1", "Unobody")
	if ok || err != nil {
		t.Errorf("DeleteLatestByUser on empty = %v, %v; want false, nil", ok, err)
	}
}

func testConcurrentAdds(t *testing.T, s ledger.Store) {
	const n = 20
	var wg sync.WaitGroup
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := s.Add(context.Background(), []ledger.Entry{entry("C1", "U1", 1, base)}); err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()
	got := list(t, s, "C1", base, base.Add(time.Second))
	if len(got) != n {
		t.Errorf("got %d entries, want %d", len(got), n)
	}
	seen := map[int64]bool{}
	for _, e := range got {
		if seen[e.ID] {
			t.Errorf("duplicate ID %d", e.ID)
		}
		seen[e.ID] = true
	}
}
