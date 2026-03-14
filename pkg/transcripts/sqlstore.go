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

// Message is a single transcript message returned by Fetcher.
type Message struct {
	SessionID string
	Role      string
	Text      string
	At        time.Time
	Seq       int64
}

// Fetcher retrieves transcript messages for a session.
type Fetcher interface {
	GetMessages(ctx context.Context, sessionID string) ([]Message, error)
}

// SQLStore is a database/sql-backed transcript store.
type SQLStore struct {
	db     *sql.DB
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
	return &SQLStore{
		db:     db,
		driver: driver,
		table:  table,
	}, nil
}

// Close closes the underlying *sql.DB.
func (s *SQLStore) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *SQLStore) SaveMessage(ctx context.Context, sessionID, role, text string, at time.Time, seq int64) error {
	// A nil store or missing DB indicates a programming/configuration error.
	// Return an explicit error so callers can detect and surface it.
	if s == nil {
		return fmt.Errorf("transcripts: SaveMessage called on nil *SQLStore")
	}
	if s.db == nil {
		return fmt.Errorf("transcripts: SaveMessage called with uninitialized *sql.DB")
	}
	if sessionID == "" || role == "" || text == "" {
		return nil
	}

	var query string
	switch s.driver {
	case "postgres":
		query = fmt.Sprintf("INSERT INTO %s (session_id, role, text, seq, created_at) VALUES ($1, $2, $3, $4, $5)", s.table)
	default: // assume MySQL-compatible placeholders
		query = fmt.Sprintf("INSERT INTO %s (session_id, role, text, seq, created_at) VALUES (?, ?, ?, ?, ?)", s.table)
	}
	_, err := s.db.ExecContext(ctx, query, sessionID, role, text, seq, at)
	if err != nil {
		return fmt.Errorf("transcripts: insert message: %w", err)
	}
	return nil
}

// GetMessages returns all transcript messages for the session, ordered by seq.
// Implements Fetcher.
func (s *SQLStore) GetMessages(ctx context.Context, sessionID string) ([]Message, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("transcripts: GetMessages called on nil or uninitialized *SQLStore")
	}
	var query string
	switch s.driver {
	case "postgres":
		query = fmt.Sprintf("SELECT session_id, role, text, created_at, seq FROM %s WHERE session_id = $1 ORDER BY seq", s.table)
	default:
		query = fmt.Sprintf("SELECT session_id, role, text, created_at, seq FROM %s WHERE session_id = ? ORDER BY seq", s.table)
	}
	rows, err := s.db.QueryContext(ctx, query, sessionID)
	if err != nil {
		return nil, fmt.Errorf("transcripts: query messages: %w", err)
	}
	defer rows.Close()
	var out []Message
	for rows.Next() {
		var m Message
		var at time.Time
		if err := rows.Scan(&m.SessionID, &m.Role, &m.Text, &at, &m.Seq); err != nil {
			return nil, fmt.Errorf("transcripts: scan message: %w", err)
		}
		m.At = at
		out = append(out, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("transcripts: iterate messages: %w", err)
	}
	return out, nil
}

