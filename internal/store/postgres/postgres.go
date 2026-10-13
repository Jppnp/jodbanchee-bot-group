// Package postgres is a ledger.Store backed by PostgreSQL.
//
// Use it on platforms without a persistent disk, such as Cloud Run. It works
// with any Postgres 13+ server, including serverless ones like Neon.
package postgres

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/lib/pq" // pure-Go driver with no transitive dependencies

	"github.com/Jppnp/jodbanchee-bot-group/internal/ledger"
	"github.com/Jppnp/jodbanchee-bot-group/internal/money"
)

// Store implements ledger.Store.
type Store struct {
	db *sql.DB
}

var _ ledger.Store = (*Store)(nil)

// Open connects to databaseURL (e.g. "postgres://user:pass@host/db?sslmode=require")
// and applies any pending schema migrations.
func Open(ctx context.Context, databaseURL string) (*Store, error) {
	connector, err := pq.NewConnector(normalizeURL(databaseURL))
	if err != nil {
		return nil, fmt.Errorf("postgres: parse DATABASE_URL: %w", err)
	}
	db := sql.OpenDB(connector)

	// Small pool: a family bot handles a few requests a minute. Serverless
	// databases (Neon) drop idle connections, so recycle them before that.
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxIdleTime(4 * time.Minute)
	db.SetConnMaxLifetime(30 * time.Minute)

	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("postgres: connect: %w", err)
	}
	if err := migrate(ctx, db); err != nil {
		db.Close()
		return nil, err
	}
	return &Store{db: db}, nil
}

// unsupportedParams are libpq options that lib/pq does not understand. lib/pq
// would forward them to the server as settings, and the server rejects them.
// Neon's copy-paste connection strings include channel_binding=require.
var unsupportedParams = []string{"channel_binding"}

// normalizeURL drops unsupportedParams from a postgres:// URL. Other DSN
// forms are returned unchanged.
func normalizeURL(databaseURL string) string {
	u, err := url.Parse(databaseURL)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return databaseURL
	}
	q := u.Query()
	for _, p := range unsupportedParams {
		q.Del(p)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// Close releases the connection pool.
func (s *Store) Close() error { return s.db.Close() }

// Ping checks the database is reachable.
func (s *Store) Ping(ctx context.Context) error { return s.db.PingContext(ctx) }

// Add implements ledger.Store. The batch is inserted in one statement, so it
// is saved completely or not at all.
func (s *Store) Add(ctx context.Context, entries []ledger.Entry) ([]ledger.Entry, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	var q strings.Builder
	q.WriteString("INSERT INTO entries (chat_id, user_id, kind, amount, note, created_at) VALUES ")
	args := make([]any, 0, len(entries)*6)
	for i, e := range entries {
		if i > 0 {
			q.WriteString(", ")
		}
		n := i * 6
		fmt.Fprintf(&q, "($%d, $%d, $%d, $%d, $%d, $%d)", n+1, n+2, n+3, n+4, n+5, n+6)
		args = append(args, e.ChatID, e.UserID, string(e.Kind), int64(e.Amount), e.Note, e.CreatedAt.UTC())
	}
	q.WriteString(" RETURNING id")

	rows, err := s.db.QueryContext(ctx, q.String(), args...)
	if err != nil {
		return nil, fmt.Errorf("postgres: insert entries: %w", err)
	}
	defer rows.Close()

	ids := make([]int64, 0, len(entries))
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("postgres: insert entries: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: insert entries: %w", err)
	}
	if len(ids) != len(entries) {
		return nil, fmt.Errorf("postgres: insert entries: got %d ids for %d rows", len(ids), len(entries))
	}

	// Postgres does not promise RETURNING order for multi-row inserts, but
	// BIGSERIAL values are assigned in VALUES order, so sorting restores it.
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	saved := make([]ledger.Entry, len(entries))
	for i, e := range entries {
		e.ID = ids[i]
		e.CreatedAt = e.CreatedAt.UTC()
		saved[i] = e
	}
	return saved, nil
}

// List implements ledger.Store.
func (s *Store) List(ctx context.Context, chatID string, from, to time.Time) ([]ledger.Entry, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, chat_id, user_id, kind, amount, note, created_at
		FROM entries
		WHERE chat_id = $1 AND created_at >= $2 AND created_at < $3
		ORDER BY created_at, id`,
		chatID, from.UTC(), to.UTC())
	if err != nil {
		return nil, fmt.Errorf("postgres: list entries: %w", err)
	}
	defer rows.Close()

	var out []ledger.Entry
	for rows.Next() {
		e, err := scanEntry(rows)
		if err != nil {
			return nil, fmt.Errorf("postgres: list entries: %w", err)
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("postgres: list entries: %w", err)
	}
	return out, nil
}

// DeleteLatestByUser implements ledger.Store.
func (s *Store) DeleteLatestByUser(ctx context.Context, chatID, userID string) (ledger.Entry, bool, error) {
	row := s.db.QueryRowContext(ctx, `
		DELETE FROM entries
		WHERE id = (
			SELECT id FROM entries
			WHERE chat_id = $1 AND user_id = $2
			ORDER BY id DESC
			LIMIT 1
		)
		RETURNING id, chat_id, user_id, kind, amount, note, created_at`,
		chatID, userID)

	e, err := scanEntry(row)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return ledger.Entry{}, false, nil
	case err != nil:
		return ledger.Entry{}, false, fmt.Errorf("postgres: delete latest entry: %w", err)
	}
	return e, true, nil
}

type scanner interface{ Scan(dest ...any) error }

func scanEntry(sc scanner) (ledger.Entry, error) {
	var (
		e      ledger.Entry
		kind   string
		amount int64
	)
	if err := sc.Scan(&e.ID, &e.ChatID, &e.UserID, &kind, &amount, &e.Note, &e.CreatedAt); err != nil {
		return ledger.Entry{}, err
	}
	e.Kind = ledger.Kind(kind)
	e.Amount = money.Satang(amount)
	e.CreatedAt = e.CreatedAt.UTC()
	return e, nil
}

//go:embed migrations/*.sql
var migrationFiles embed.FS

// migrationLockID is an arbitrary constant for pg_advisory_lock, so instances
// starting at the same time (Cloud Run scaling up) run migrations one by one.
const migrationLockID = 4_271_530_917

// migrate applies migrations/NNN_name.sql files that have not run yet, in
// order, each in its own transaction.
func migrate(ctx context.Context, db *sql.DB) error {
	names, err := fs.Glob(migrationFiles, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(names)

	// Advisory locks belong to a session, so hold one connection throughout.
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("postgres: migrate: %w", err)
	}
	defer conn.Close()
	if _, err := conn.ExecContext(ctx, "SELECT pg_advisory_lock($1)", migrationLockID); err != nil {
		return fmt.Errorf("postgres: migrate: lock: %w", err)
	}
	defer conn.ExecContext(context.WithoutCancel(ctx), "SELECT pg_advisory_unlock($1)", migrationLockID)

	if _, err := conn.ExecContext(ctx, `
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INT PRIMARY KEY,
			applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
		)`); err != nil {
		return fmt.Errorf("postgres: create schema_migrations: %w", err)
	}

	for _, name := range names {
		base := strings.TrimPrefix(name, "migrations/")
		prefix, _, _ := strings.Cut(base, "_")
		version, err := strconv.Atoi(prefix)
		if err != nil {
			return fmt.Errorf("postgres: migration %s: name must start with a number", base)
		}
		body, err := migrationFiles.ReadFile(name)
		if err != nil {
			return err
		}
		if err := applyMigration(ctx, conn, version, string(body)); err != nil {
			return fmt.Errorf("postgres: migration %s: %w", base, err)
		}
	}
	return nil
}

func applyMigration(ctx context.Context, conn *sql.Conn, version int, body string) error {
	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() // no-op after Commit

	var done bool
	if err := tx.QueryRowContext(ctx,
		"SELECT EXISTS (SELECT 1 FROM schema_migrations WHERE version = $1)", version).Scan(&done); err != nil {
		return err
	}
	if done {
		return nil
	}
	if _, err := tx.ExecContext(ctx, body); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, "INSERT INTO schema_migrations (version) VALUES ($1)", version); err != nil {
		return err
	}
	return tx.Commit()
}
