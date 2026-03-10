package transcripts

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"time"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

// Store persists per-message transcripts for a session.
type Store interface {
	SaveMessage(ctx context.Context, sessionID, role, text string, at time.Time, seq int64) error
	Close() error
}

const (
	defaultMaxOpenConns    = 25
	defaultMaxIdleConns    = 10
	defaultConnMaxLifetime  = time.Hour
)

// SQLStore is a database/sql-backed transcript store.
type SQLStore struct {
	db     *sql.DB
	stmt   *sql.Stmt
	driver string
	table  string
}

var tableNameRe = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)

// NewSQLStore creates a new SQL-backed Store for the given driver and DSN.
// driver should be "postgres" or "mysql"; table is the table name for inserts.
func NewSQLStore(driver, dsn, table string) (*SQLStore, error) {
	if driver == "" {
		return nil, fmt.Errorf("transcripts: driver is required")
	}
	if dsn == "" {
		return nil, fmt.Errorf("transcripts: dsn is required")
	}
	if table == "" {
		table = "call_transcripts"
	}
	if !tableNameRe.MatchString(table) {
		return nil, fmt.Errorf("transcripts: invalid table name %q", table)
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("transcripts: open db: %w", err)
	}
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("transcripts: ping db: %w", err)
	}
	db.SetMaxOpenConns(defaultMaxOpenConns)
	db.SetMaxIdleConns(defaultMaxIdleConns)
	db.SetConnMaxLifetime(defaultConnMaxLifetime)

	var query string
	switch driver {
	case "postgres":
		query = fmt.Sprintf("INSERT INTO %s (session_id, role, text, seq, created_at) VALUES ($1, $2, $3, $4, $5)", table)
	default:
		query = fmt.Sprintf("INSERT INTO %s (session_id, role, text, seq, created_at) VALUES (?, ?, ?, ?, ?)", table)
	}
	stmt, err := db.Prepare(query)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("transcripts: prepare: %w", err)
	}
	return &SQLStore{
		db:     db,
		stmt:   stmt,
		driver: driver,
		table:  table,
	}, nil
}

// Close closes the prepared statement and the underlying *sql.DB.
func (s *SQLStore) Close() error {
	if s == nil {
		return nil
	}
	if s.stmt != nil {
		_ = s.stmt.Close()
		s.stmt = nil
	}
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *SQLStore) SaveMessage(ctx context.Context, sessionID, role, text string, at time.Time, seq int64) error {
	if s == nil {
		return fmt.Errorf("transcripts: SaveMessage called on nil *SQLStore")
	}
	if s.stmt == nil {
		return fmt.Errorf("transcripts: SaveMessage called with uninitialized store")
	}
	if sessionID == "" || role == "" || text == "" {
		return nil
	}
	_, err := s.stmt.ExecContext(ctx, sessionID, role, text, seq, at)
	if err != nil {
		return fmt.Errorf("transcripts: insert message: %w", err)
	}
	return nil
}

