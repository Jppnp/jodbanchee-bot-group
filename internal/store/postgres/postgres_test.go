package postgres

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Jppnp/jodbanchee-bot-group/internal/ledger"
	"github.com/Jppnp/jodbanchee-bot-group/internal/store/storetest"
)

// These tests need a real database. Point TEST_DATABASE_URL at a throwaway
// one, e.g.:
//
//	docker run --rm -p 5432:5432 -e POSTGRES_PASSWORD=pg postgres:17
//	TEST_DATABASE_URL='postgres://postgres:pg@localhost:5432/postgres?sslmode=disable' go test ./...
//
// The tests drop and recreate the schema, so never use a real database.
func testURL(t *testing.T) string {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	return url
}

// freshStore resets the schema and opens a store on it.
func freshStore(t *testing.T) *Store {
	t.Helper()
	ctx := context.Background()
	url := testURL(t)

	s, err := Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, "DROP TABLE IF EXISTS entries, schema_migrations"); err != nil {
		t.Fatal(err)
	}
	s.Close()

	s, err = Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestContract(t *testing.T) {
	storetest.Run(t, func(t *testing.T) ledger.Store { return freshStore(t) })
}

func TestMigrationsAreIdempotent(t *testing.T) {
	freshStore(t)
	ctx := context.Background()

	// Several instances starting at once (e.g. Cloud Run scaling up) must not
	// fail or apply a migration twice.
	var wg sync.WaitGroup
	errs := make(chan error, 5)
	for range 5 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := Open(ctx, testURL(t))
			if err != nil {
				errs <- err
				return
			}
			s.Close()
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}

	s, err := Open(ctx, testURL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var n int
	if err := s.db.QueryRowContext(ctx, "SELECT count(*) FROM schema_migrations").Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("schema_migrations has %d rows, want 1", n)
	}
}

func TestBatchIsAtomic(t *testing.T) {
	s := freshStore(t)
	ctx := context.Background()
	at := time.Now().UTC()
	good := ledger.Entry{ChatID: "C1", Kind: ledger.Income, Amount: 100, CreatedAt: at}
	bad := good
	bad.Amount = 0 // violates CHECK (amount > 0)

	if _, err := s.Add(ctx, []ledger.Entry{good, bad}); err == nil {
		t.Fatal("expected constraint error")
	}
	got, err := s.List(ctx, "C1", at.Add(-time.Second), at.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("half of a failed batch was saved: %+v", got)
	}
}

func TestOpenRejectsBadURL(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := Open(ctx, "postgres://nobody@127.0.0.1:1/x?sslmode=disable&connect_timeout=2"); err == nil {
		t.Error("expected connection error")
	}
}

func TestNormalizeURL(t *testing.T) {
	tests := map[string]string{
		// Neon's copy-paste string: channel_binding dropped, the rest kept.
		"postgresql://u:p@ep-x.ap-southeast-1.aws.neon.tech/db?sslmode=require&channel_binding=require": "postgresql://u:p@ep-x.ap-southeast-1.aws.neon.tech/db?sslmode=require",
		"postgres://u@localhost/db?sslmode=disable":                                                     "postgres://u@localhost/db?sslmode=disable",
		"host=localhost dbname=db":                                                                      "host=localhost dbname=db", // key=value form untouched
	}
	for in, want := range tests {
		if got := normalizeURL(in); got != want {
			t.Errorf("normalizeURL(%q)\n got  %q\n want %q", in, got, want)
		}
	}
}

func TestOpenAcceptsNeonStyleURL(t *testing.T) {
	freshStore(t)
	s, err := Open(context.Background(), testURL(t)+"&channel_binding=require")
	if err != nil {
		t.Fatalf("Open with channel_binding: %v", err)
	}
	s.Close()
}
